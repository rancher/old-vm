package vm

import (
	"fmt"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

func (ctrl *VirtualMachineController) updateNovnc(vm *vmapi.VirtualMachine, podName string) (err error) {
	if vm.Spec.HostedNovnc {
		if err = ctrl.updateNovncPod(vm, podName); err != nil {
			glog.Warningf("error updating novnc pod %s: %v", vm.Name, err)
		}
		if err = ctrl.updateNovncService(vm); err != nil {
			glog.Warningf("error updating novnc service %s: %v", vm.Name, err)
		}
	} else {
		if err = ctrl.deleteNovncPod(vm.Name); err != nil && !apierrors.IsNotFound(err) {
			glog.Warningf("error deleting novnc pod %s: %v", vm.Name, err)
		}
		if err = ctrl.deleteNovncService(vm.Name); err != nil && !apierrors.IsNotFound(err) {
			glog.Warningf("error deleting novnc service %s: %v", vm.Name, err)
		}
		vm2 := vm.DeepCopy()
		vm2.Status.VncEndpoint = ""
		if err = ctrl.updateVMStatus(vm, vm2); err != nil {
			glog.Warningf("error removing vnc endpoint from vm %s/%s: %v", common.NamespaceVM, vm.Name, err)
		}
	}
	return
}

func (ctrl *VirtualMachineController) updateNovncPod(vm *vmapi.VirtualMachine, podName string) error {
	pod, err := ctrl.podLister.Pods(common.NamespaceVM).Get(vm.Name + "-novnc")
	switch {
	case !apierrors.IsNotFound(err):
		return err
	case err == nil:
		if pod.DeletionTimestamp == nil {
			return nil
		}
		fallthrough
	default:
		pod, err = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(makeNovncPod(vm, podName))
	}
	return err
}

func (ctrl *VirtualMachineController) updateNovncService(vm *vmapi.VirtualMachine) error {
	svc, err := ctrl.svcLister.Services(common.NamespaceVM).Get(vm.Name + "-novnc")
	switch {
	case err == nil:
		break
	case apierrors.IsNotFound(err):
		if svc, err = ctrl.kubeClient.CoreV1().Services(common.NamespaceVM).Create(makeNovncService(vm)); err != nil {
			return err
		}
	default:
		return err
	}

	switch {
	case vm.Status.NodeIP == "":
		return nil
	case len(svc.Spec.Ports) != 1:
		return nil
	case svc.Spec.Ports[0].NodePort <= 0:
		return nil
	}

	vm2 := vm.DeepCopy()
	vm2.Status.VncEndpoint = fmt.Sprintf("%s:%d", vm.Status.NodeIP, svc.Spec.Ports[0].NodePort)
	return ctrl.updateVMStatus(vm, vm2)
}

func (ctrl *VirtualMachineController) deleteNovncPod(name string) error {
	_, err := ctrl.podLister.Pods(common.NamespaceVM).Get(name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("trying to delete novnc pod %s", name)
		return ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(name+"-novnc", &metav1.DeleteOptions{})
	default:
		return err
	}
}

func (ctrl *VirtualMachineController) deleteNovncService(name string) error {
	_, err := ctrl.svcLister.Services(common.NamespaceVM).Get(name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("trying to delete novnc service %s", name)
		return ctrl.kubeClient.CoreV1().Services(common.NamespaceVM).Delete(name+"-novnc", &metav1.DeleteOptions{})
	default:
		return err
	}
}
