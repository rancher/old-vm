package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
	"github.com/rancher/vm/pkg/qemu"
)

func (ctrl *VirtualMachineController) migrateVM(vm *vmapi.VirtualMachine) error {
	switch vm.Status.State {
	case vmapi.StateRunning:
		vm2 := vm.DeepCopy()
		vm2.Status.State = vmapi.StateMigrating
		if err := ctrl.updateVMStatus(vm, vm2); err != nil {
			return err
		}
		vm = vm2
	case vmapi.StateMigrating:
		break
	default:
		return errors.New(fmt.Sprintf("Migration unimplemented for VM in %s state", vm.Status.State))
	}

	ready, oldPod, newPod, err := ctrl.startMigrateTargetPod(vm)
	if err != nil {
		return err
	}

	// Check if the user canceled mid-migration
	if oldPod != nil && vm.Spec.NodeName == oldPod.Spec.NodeName && vm.Status.State == vmapi.StateMigrating {
		glog.V(2).Infof("User canceled migration of vm %s", vm.Name)
		return ctrl.migrateRollback(vm, newPod)
	}

	if !ready {
		return nil
	}

	succeeded, err := ctrl.runMigrationJob(vm, oldPod, newPod)
	if err != nil || !succeeded {
		return err
	}

	if err := ctrl.migrationCleanup(vm, oldPod, newPod); err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) migrateRollback(vm *vmapi.VirtualMachine, pod *corev1.Pod) error {
	if err := ctrl.deleteMigrationJob(vm); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	vm2 := vm.DeepCopy()
	vm2.Status.State = vmapi.StateRunning
	return ctrl.updateVMStatus(vm, vm2)
}

var fg = metav1.DeletePropagationForeground

func (ctrl *VirtualMachineController) deleteMigrationJob(vm *vmapi.VirtualMachine) error {
	return ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Delete(
		getJobName(vm),
		&metav1.DeleteOptions{
			PropagationPolicy: &fg,
		})
}

func getJobName(vm *vmapi.VirtualMachine) string {
	return fmt.Sprintf("%s-migrate", vm.Name)
}

func (ctrl *VirtualMachineController) migrationCleanup(vm *vmapi.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) error {
	vm2 := vm.DeepCopy()
	vm2.Spec.Action = vmapi.ActionStart
	if err := ctrl.updateVMStatusWithPod(vm, vm2, newPod); err != nil {
		return err
	}

	if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(oldPod.Name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := ctrl.deleteMigrationJob(vm); err != nil {
		return err
	}

	// novnc pod needs to move at this point to read from unix socket on different host
	return ctrl.deleteConsolePod(vm)
}

func (ctrl *VirtualMachineController) startMigrateTargetPod(vm *vmapi.VirtualMachine) (bool, *corev1.Pod, *corev1.Pod, error) {

	// List vm pods
	pods, err := ctrl.podLister.Pods(common.NamespaceVM).List(labels.Set{
		"app":  common.LabelApp,
		"name": vm.Name,
		"role": common.LabelRoleVM,
	}.AsSelector())

	if err != nil {
		return false, nil, nil, err
	}

	alivePods := GetAlivePods(pods)
	switch len(alivePods) {
	// If the second pod doesn't already exist, start one.
	case 1:
		var getErr, createErr error
		var pod *corev1.Pod
		if vm.Spec.Volume.Longhorn != nil {
			pod = ctrl.createLonghornMachinePod(vm, true)
		} else {
			pod = ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, true)
		}
		pod, createErr = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod)
		if createErr != nil {
			glog.V(2).Infof("Error creating vm pod %s/%s: %v", common.NamespaceVM, vm.Name, createErr)
			return false, nil, nil, createErr
		}
		// Get the created pod into cache for the next poll
		pod, getErr = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Get(pod.Name, metav1.GetOptions{})
		return false, nil, nil, getErr

	// Suspend the migration procedure until both pods enter running
	// state. Pod phase changes trigger requeueing, so this is safe.
	case 2:
		oldPod := alivePods[0]
		newPod := alivePods[1]
		if !oldPod.CreationTimestamp.Before(&(newPod.CreationTimestamp)) {
			oldPod = alivePods[1]
			newPod = alivePods[0]
		}
		for _, pod := range alivePods {
			if !common.IsPodReady(pod) {
				return false, oldPod, newPod, nil
			}
		}
		return true, oldPod, newPod, nil
	}

	return false, nil, nil, errors.New(fmt.Sprintf("strange number of vm pods found for %s: %d", vm.Name, len(pods)))
}

func (ctrl *VirtualMachineController) runMigrationJob(vm *vmapi.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) (bool, error) {
	job, err := ctrl.jobLister.Jobs(common.NamespaceVM).Get(getJobName(vm))

	switch {
	case err == nil:
		return job.Status.Succeeded == 1, nil

	case apierrors.IsNotFound(err):
		migratePort, ok := newPod.ObjectMeta.Annotations["migrate_port"]
		if !ok {
			return false, errors.New(fmt.Sprintf("Missing migrate_port annotation on migration pod for vm %s", vm.Name))
		}
		job := qemu.NewMigrationJob(vm, oldPod.Name, fmt.Sprintf("tcp:%s:%s", newPod.Status.PodIP, migratePort))
		job, err = ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Create(job)
		return false, err
	}

	return false, errors.New(fmt.Sprintf("error getting job from lister for vm %s: %v", vm.Name, err))
}
