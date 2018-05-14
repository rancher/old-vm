package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
	"github.com/rancher/vm/pkg/qemu"
)

func (ctrl *VirtualMachineController) migrateVM(vm *vmapi.VirtualMachine) (err error) {
	// We currently only support live migration.
	if vm.Status.State != vmapi.StateRunning {
		return errors.New(fmt.Sprintf("Migration unimplemented for VM in %s state", vm.Status.State))
	}

	ready, oldPod, newPod, err := ctrl.startMigrateTargetPod(vm)
	if err != nil || !ready {
		return err
	}

	completed, err := ctrl.startMigrationJob(vm, oldPod, newPod)
	if err != nil || !completed {
		return err
	}

	if err := ctrl.migrationCleanup(vm, oldPod); err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) migrationCleanup(vm *vmapi.VirtualMachine, oldPod *corev1.Pod) error {
	glog.V(5).Infof("migrationCleanup")

	err := ctrl.kubeClient.BatchV1().Jobs(NamespaceVM).Delete(
		fmt.Sprintf("%s-migrate", vm.Name), &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = ctrl.kubeClient.CoreV1().Pods(NamespaceVM).Delete(oldPod.Name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) startMigrateTargetPod(vm *vmapi.VirtualMachine) (bool, *corev1.Pod, *corev1.Pod, error) {
	glog.V(5).Infof("startMigrationTargetPod")

	// List vm pods
	pods, err := ctrl.podLister.Pods(NamespaceVM).List(labels.Set{
		"app":  "ranchervm",
		"name": vm.Name,
		"role": "vm",
	}.AsSelector())

	if err != nil {
		return false, nil, nil, err
	}

	switch len(pods) {
	// If the second pod doesn't already exist, start one.
	case 1:
		_, err = ctrl.kubeClient.CoreV1().Pods(NamespaceVM).Create(ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, true))
		if err != nil {
			glog.V(2).Infof("Error creating vm pod %s/%s: %v", NamespaceVM, vm.Name, err)
			return false, nil, nil, err
		}
		return false, nil, nil, nil

	// Suspend the migration procedure until both pods enter running
	// state. Pod phase changes trigger requeueing, so this is safe.
	case 2:
		for _, pod := range pods {
			if !common.IsPodReady(pod) {
				return false, nil, nil, nil
			}
		}

		oldPod := pods[0]
		newPod := pods[1]
		if !oldPod.CreationTimestamp.Before(&(newPod.CreationTimestamp)) {
			oldPod = pods[1]
			newPod = pods[0]
		}

		return true, oldPod, newPod, nil
	}

	return false, nil, nil, errors.New(fmt.Sprintf("strange number of vm pods found for %s: %d", vm.Name, len(pods)))
}

func (ctrl *VirtualMachineController) startMigrationJob(vm *vmapi.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) (bool, error) {
	glog.V(5).Infof("startMigrationJob")

	// List migration pods
	pods, err := ctrl.podLister.Pods(NamespaceVM).List(labels.Set{
		"app":  "ranchervm",
		"name": vm.Name,
		"role": "migrate",
	}.AsSelector())

	if err != nil {
		return false, err
	}

	switch len(pods) {
	// If the migration job pod doesn't exist, start one.
	case 0:
		if migratePort, ok := newPod.ObjectMeta.Annotations["migrate_port"]; ok {
			migrationJob := qemu.NewMigrationJob(vm, oldPod.Name, fmt.Sprintf("tcp:%s:%s", newPod.Status.PodIP, migratePort))
			if _, err = ctrl.kubeClient.BatchV1().Jobs(NamespaceVM).Create(migrationJob); err != nil {
				return false, err
			}
		} else {
			return false, errors.New(fmt.Sprintf("Missing migrate_port annotation on migration pod for vm %s", vm.Name))
		}

	// Suspend migration procedure until migrate job pod enters completed state
	case 1:
		glog.Infof("phase: %+v", pods[0].Status.Phase)
		if pods[0].Status.Phase != corev1.PodSucceeded {
			return false, nil
		}
		return true, nil
	}

	return false, nil
}
