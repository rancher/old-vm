package vm

import (
	"fmt"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

// FIXME shouldn't be hardcoded
const NODE_HOSTNAME = "kvm.local"

func (ctrl *VirtualMachineController) updateNovnc(vm *vmapi.VirtualMachine) (err error) {
	if vm.Spec.HostedNovnc {
		if err = ctrl.updateNovncPod(vm); err != nil {
			glog.Warningf("error updating novnc pod %s/%s: %v", NamespaceVM, vm.Name, err)
		}
		if err = ctrl.updateNovncService(vm); err != nil {
			glog.Warningf("error updating novnc service %s/%s: %v", NamespaceVM, vm.Name, err)
		}
	} else {
		if err = ctrl.deleteNovncPod(NamespaceVM, vm.Name); err != nil {
			glog.Warningf("error deleting novnc pod %s/%s: %v", NamespaceVM, vm.Name, err)
		}
		if err = ctrl.deleteNovncService(NamespaceVM, vm.Name); err != nil {
			glog.Warningf("error deleting novnc service %s/%s: %v", NamespaceVM, vm.Name, err)
		}
		vm2 := vm.DeepCopy()
		vm2.Status.VncEndpoint = ""
		if err = ctrl.updateVMStatus(vm, vm2); err != nil {
			glog.Warningf("error removing vnc endpoint from vm %s/%s: %v", NamespaceVM, vm.Name, err)
		}
	}
	return
}

func (ctrl *VirtualMachineController) updateNovncPod(vm *vmapi.VirtualMachine) (err error) {
	pod, err := ctrl.podLister.Pods(NamespaceVM).Get(vm.Name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("Found existing novnc pod %s/%s", pod.Namespace, pod.Name)
	case !apierrors.IsNotFound(err):
		glog.V(2).Infof("error getting novnc pod %s/%s: %v", NamespaceVM, vm.Name, err)
		return
	default:
		_, err = ctrl.kubeClient.CoreV1().Pods(NamespaceVM).Create(makeNovncPod(vm))
		if err != nil {
			glog.V(2).Infof("Error creating novnc pod %s/%s: %v", NamespaceVM, vm.Name, err)
			return
		}
	}
	return
}

func (ctrl *VirtualMachineController) updateNovncService(vm *vmapi.VirtualMachine) (err error) {
	vm2 := vm.DeepCopy()

	svc, err := ctrl.svcLister.Services(NamespaceVM).Get(vm.Name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("Found existing novnc service %s/%s", svc.Namespace, svc.Name)
		vm2.Status.VncEndpoint = fmt.Sprintf("%s:%d", NODE_HOSTNAME, svc.Spec.Ports[0].NodePort)
	case !apierrors.IsNotFound(err):
		glog.V(2).Infof("error getting novnc service %s/%s: %v", NamespaceVM, vm.Name, err)
		return
	default:
		svc, err = ctrl.kubeClient.CoreV1().Services(NamespaceVM).Create(makeNovncService(vm))
		if err != nil {
			glog.V(2).Infof("Error creating novnc service %s/%s: %v", NamespaceVM, vm.Name, err)
			return
		}
		vm2.Status.VncEndpoint = fmt.Sprintf("%s:%d", NODE_HOSTNAME, svc.Spec.Ports[0].NodePort)
	}

	err = ctrl.updateVMStatus(vm, vm2)
	return
}

func (ctrl *VirtualMachineController) deleteNovncPod(ns, name string) error {
	_, err := ctrl.podLister.Pods(ns).Get(name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("trying to delete novnc pod %s/%s", ns, name)
		return ctrl.kubeClient.CoreV1().Pods(ns).Delete(name+"-novnc", &metav1.DeleteOptions{})
	default:
		return err
	}
}

func (ctrl *VirtualMachineController) deleteNovncService(ns, name string) error {
	_, err := ctrl.svcLister.Services(ns).Get(name + "-novnc")
	switch {
	case err == nil:
		glog.V(2).Infof("trying to delete novnc service %s/%s", ns, name)
		return ctrl.kubeClient.CoreV1().Services(ns).Delete(name+"-novnc", &metav1.DeleteOptions{})
	default:
		return err
	}
}
