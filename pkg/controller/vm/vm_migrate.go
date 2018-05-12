package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
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

	ready, oldPod, _, err := ctrl.startMigrationPod(vm)
	if err != nil || !ready {
		return err
	}

	// FIXME need to derive the endpoint from newPod
	migrationJob := qemu.NewMigrationJob(vm, oldPod.Name, "tcp:172.16.58.187:4444")
	_, err = ctrl.kubeClient.BatchV1().Jobs(NamespaceVM).Create(migrationJob)
	if err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) startMigrationPod(vm *vmapi.VirtualMachine) (bool, *corev1.Pod, *corev1.Pod, error) {
	// List all pods belonging to the VM
	pods, err := ctrl.podLister.Pods(NamespaceVM).List(labels.Set{
		"app":  "ranchervm",
		"name": vm.Name,
		"role": "vm",
	}.AsSelector())

	// If an error listing pods occurs, break out.
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
