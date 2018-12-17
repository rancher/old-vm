package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
	"github.com/rancher/vm/pkg/qemu"
)

func (ctrl *VirtualMachineController) migrateMachine(machine *api.VirtualMachine) error {
	switch machine.Status.State {
	case api.StateRunning:
		machine2 := machine.DeepCopy()
		machine2.Status.State = api.StateMigrating
		if err := ctrl.updateMachineStatus(machine, machine2); err != nil {
			return err
		}
		machine = machine2
	case api.StateMigrating:
		break
	default:
		return errors.New(fmt.Sprintf("Migration unimplemented for VM in %s state", machine.Status.State))
	}

	ready, oldPod, newPod, err := ctrl.startMigrateTargetPod(machine)
	if err != nil {
		return err
	}

	// Check if the user canceled mid-migration
	if oldPod != nil && machine.Spec.NodeName == oldPod.Spec.NodeName && machine.Status.State == api.StateMigrating {
		glog.V(2).Infof("User canceled migration of machine %s", machine.Name)
		return ctrl.migrateRollback(machine, newPod)
	}

	if !ready {
		return nil
	}

	succeeded, err := ctrl.runMigrationJob(machine, oldPod, newPod)
	if err != nil || !succeeded {
		return err
	}

	if err := ctrl.migrationCleanup(machine, oldPod, newPod); err != nil {
		return err
	}

	return nil
}

func (ctrl *VirtualMachineController) migrateRollback(machine *api.VirtualMachine, pod *corev1.Pod) error {
	if err := ctrl.deleteMigrationJob(machine); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	machine2 := machine.DeepCopy()
	machine2.Status.State = api.StateRunning
	return ctrl.updateMachineStatus(machine, machine2)
}

var fg = metav1.DeletePropagationForeground

func (ctrl *VirtualMachineController) deleteMigrationJob(machine *api.VirtualMachine) error {
	return ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Delete(
		getJobName(machine),
		&metav1.DeleteOptions{
			PropagationPolicy: &fg,
		})
}

func getJobName(machine *api.VirtualMachine) string {
	return fmt.Sprintf("%s-migrate", machine.Name)
}

func (ctrl *VirtualMachineController) migrationCleanup(machine *api.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) error {
	machine2 := machine.DeepCopy()
	machine2.Spec.Action = api.ActionStart
	if err := ctrl.updateMachineStatusWithPod(machine, machine2, newPod); err != nil {
		return err
	}

	if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(oldPod.Name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := ctrl.deleteMigrationJob(machine); err != nil {
		return err
	}

	// novnc pod needs to move at this point to read from unix socket on different host
	return ctrl.deleteConsolePod(machine)
}

func (ctrl *VirtualMachineController) startMigrateTargetPod(machine *api.VirtualMachine) (bool, *corev1.Pod, *corev1.Pod, error) {
	image, err := ctrl.machineImageLister.Get(machine.Spec.MachineImage)
	if err != nil {
		return false, nil, nil, err
	}

	if image.Status.State != api.MachineImageReady {
		return false, nil, nil, fmt.Errorf("Machine image state: %s", image.Status.State)
	}

	publicKeys, err := ctrl.getPublicKeys(machine)
	if err != nil {
		return false, nil, nil, err
	}

	// List machine pods
	pods, err := ctrl.podLister.Pods(common.NamespaceVM).List(labels.Set{
		"app":  common.LabelApp,
		"name": machine.Name,
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
		if machine.Spec.Volume.Longhorn != nil {
			pod = ctrl.createLonghornMachinePod(machine, publicKeys, true)
		} else {
			pod = ctrl.createMachinePod(machine, publicKeys, image, true)
		}
		pod, createErr = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod)
		if createErr != nil {
			glog.V(2).Infof("Error creating machine pod %s/%s: %v", common.NamespaceVM, machine.Name, createErr)
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

	return false, nil, nil, errors.New(fmt.Sprintf("strange number of machine pods found for %s: %d", machine.Name, len(pods)))
}

func (ctrl *VirtualMachineController) runMigrationJob(machine *api.VirtualMachine, oldPod *corev1.Pod, newPod *corev1.Pod) (bool, error) {
	job, err := ctrl.jobLister.Jobs(common.NamespaceVM).Get(getJobName(machine))

	switch {
	case err == nil:
		return job.Status.Succeeded == 1, nil

	case apierrors.IsNotFound(err):
		migratePort, ok := newPod.ObjectMeta.Annotations["migrate_port"]
		if !ok {
			return false, errors.New(fmt.Sprintf("Missing migrate_port annotation on migration pod for machine %s", machine.Name))
		}
		targetURI := fmt.Sprintf("tcp:%s:%s", newPod.Status.PodIP, migratePort)
		job := qemu.NewMigrationJob(machine, oldPod.Name, targetURI, ctrl.getImagePullSecrets())
		job, err = ctrl.kubeClient.BatchV1().Jobs(common.NamespaceVM).Create(job)
		return false, err
	}

	return false, errors.New(fmt.Sprintf("error getting job from lister for machine %s: %v", machine.Name, err))
}
