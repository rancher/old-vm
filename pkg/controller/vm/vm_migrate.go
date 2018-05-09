package vm

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

func (ctrl *VirtualMachineController) migrateVM(vm *vmapi.VirtualMachine) (err error) {
	// We currently only support live migration.
	if vm.Status.State != vmapi.StateRunning {
		return errors.New(fmt.Sprintf("Migration unimplemented for VM in %s state", vm.Status.State))
	}

	ready, err := ctrl.startMigrationPod(vm)
	if err != nil || !ready {
		return err
	}

	// TODO ctrl.startMigrationJob(vm)

	return nil
}

func (ctrl *VirtualMachineController) startMigrationPod(vm *vmapi.VirtualMachine) (bool, error) {
	// List all pods belonging to the VM
	pods, err := ctrl.podLister.Pods(NamespaceVM).List(labels.Set{
		"name": vm.Name,
	}.AsSelector())

	// If an error listing pods occurs, break out.
	if err != nil {
		return false, err
	}

	switch len(pods) {
	// If the second pod doesn't already exist, start one.
	case 1:
		_, err = ctrl.kubeClient.CoreV1().Pods(NamespaceVM).Create(ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, true))
		if err != nil {
			glog.V(2).Infof("Error creating vm pod %s/%s: %v", NamespaceVM, vm.Name, err)
			return false, err
		}
		return false, nil

	// Suspend the migration procedure until both pods enter running
	// state. Pod phase changes trigger requeueing, so this is safe.
	case 2:
		for _, pod := range pods {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}
		return true, nil
	}

	return false, errors.New(fmt.Sprintf("strange number of vm pods found for %s: %d", vm.Name, len(pods)))
}
