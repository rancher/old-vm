package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
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
		ctrl.updateVMStatus(vm, vm2)
		ctrl.migrateVM(vm2)
	case vmapi.StateMigrating:
		break
	default:
		return errors.New(fmt.Sprintf("Migration unimplemented for VM in %s state", vm.Status.State))
	}

	ready, oldPod, newPod, err := ctrl.startMigrateTargetPod(vm)
	if err != nil || !ready {
		return err
	}

	succeeded, migrateJob, err := ctrl.runMigrationJob(vm, oldPod, newPod)
	if err != nil || !succeeded {
		return err
	}

	if err := ctrl.migrationCleanup(vm, oldPod, newPod, migrateJob); err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) migrationCleanup(vm *vmapi.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod, migrateJob *batchv1.Job) error {
	vm2 := vm.DeepCopy()
	vm2.Spec.Action = vmapi.ActionStart
	if err := ctrl.updateVMStatusWithPod(vm, vm2, newPod); err != nil {
		return err
	}

	if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(oldPod.Name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	fg := metav1.DeletePropagationForeground
	if err := ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Delete(migrateJob.Name, &metav1.DeleteOptions{
		PropagationPolicy: &fg,
	}); err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) startMigrateTargetPod(vm *vmapi.VirtualMachine) (bool, *corev1.Pod, *corev1.Pod, error) {

	// List vm pods
	pods, err := ctrl.podLister.Pods(common.NamespaceVM).List(labels.Set{
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
		var getErr, createErr error
		pod := ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, true)
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

func (ctrl *VirtualMachineController) runMigrationJob(vm *vmapi.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) (bool, *batchv1.Job, error) {
	job, err := ctrl.jobLister.Jobs(common.NamespaceVM).Get(fmt.Sprintf("%s-migrate", vm.Name))

	switch {
	case err == nil:
		return job.Status.Succeeded == 1, job, nil

	case apierrors.IsNotFound(err):
		migratePort, ok := newPod.ObjectMeta.Annotations["migrate_port"]
		if !ok {
			return false, nil, errors.New(fmt.Sprintf("Missing migrate_port annotation on migration pod for vm %s", vm.Name))
		}
		job := qemu.NewMigrationJob(vm, oldPod.Name, fmt.Sprintf("tcp:%s:%s", newPod.Status.PodIP, migratePort))
		job, err = ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Create(job)
		return false, job, err
	}

	return false, nil, errors.New(fmt.Sprintf("error getting job from lister for vm %s: %v", vm.Name, err))
}
