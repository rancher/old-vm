package vm

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	vmclientset "github.com/rancher/vm/pkg/client/clientset/versioned"
	vminformers "github.com/rancher/vm/pkg/client/informers/externalversions/virtualmachine/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/virtualmachine/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

const FinalizerDeletion = "deletion.vm.rancher.com"
const NamespaceVM = "default"

type VirtualMachineController struct {
	vmClient   vmclientset.Interface
	kubeClient kubernetes.Interface

	vmLister         vmlisters.VirtualMachineLister
	vmListerSynced   cache.InformerSynced
	podLister        corelisters.PodLister
	podListerSynced  cache.InformerSynced
	svcLister        corelisters.ServiceLister
	svcListerSynced  cache.InformerSynced
	credLister       vmlisters.CredentialLister
	credListerSynced cache.InformerSynced

	vmQueue  workqueue.RateLimitingInterface
	podQueue workqueue.RateLimitingInterface

	bridgeIface      string
	noResourceLimits bool
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewVirtualMachineController(
	vmClient vmclientset.Interface,
	kubeClient kubernetes.Interface,
	vmInformer vminformers.VirtualMachineInformer,
	podInformer coreinformers.PodInformer,
	svcInformer coreinformers.ServiceInformer,
	credInformer vminformers.CredentialInformer,
	bridgeIface string,
	noResourceLimits bool,
) *VirtualMachineController {

	ctrl := &VirtualMachineController{
		vmClient:         vmClient,
		kubeClient:       kubeClient,
		vmQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "virtualmachine"),
		podQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		bridgeIface:      bridgeIface,
		noResourceLimits: noResourceLimits,
	}

	vmInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, obj) },
		},
	)

	podInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: ctrl.podFilterFunc,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.podQueue, obj) },
				UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.podQueue, newObj) },
				DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.podQueue, obj) },
			},
		},
	)

	// TODO handle service resource events

	ctrl.vmLister = vmInformer.Lister()
	ctrl.vmListerSynced = vmInformer.Informer().HasSynced

	ctrl.podLister = podInformer.Lister()
	ctrl.podListerSynced = podInformer.Informer().HasSynced

	ctrl.svcLister = svcInformer.Lister()
	ctrl.svcListerSynced = svcInformer.Informer().HasSynced

	ctrl.credLister = credInformer.Lister()
	ctrl.credListerSynced = credInformer.Informer().HasSynced

	return ctrl
}

func (ctrl *VirtualMachineController) Run(workers int, stopCh <-chan struct{}) {
	defer ctrl.vmQueue.ShutDown()

	glog.Infof("Starting vm controller")
	defer glog.Infof("Shutting down vm Controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.vmListerSynced, ctrl.podListerSynced, ctrl.svcListerSynced, ctrl.credListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ctrl.vmWorker, time.Second, stopCh)
	}
	go wait.Until(ctrl.podWorker, time.Second, stopCh)

	<-stopCh
}

func (ctrl *VirtualMachineController) enqueueWork(queue workqueue.Interface, obj interface{}) {
	// Beware of "xxx deleted" events
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("failed to get key from object: %v", err)
		return
	}
	glog.V(5).Infof("enqueued %q for sync", objName)
	queue.Add(objName)
}

func (ctrl *VirtualMachineController) updateVmPod(vm *vmapi.VirtualMachine) (err error) {
	vm2 := vm.DeepCopy()
	vm2.Status.State = vmapi.StatePending

	pods, err := ctrl.podLister.Pods(NamespaceVM).List(labels.Set{
		"app":  "ranchervm",
		"name": vm.Name,
		"role": "vm",
	}.AsSelector())

	// TODO this if/else statement needs simplifying
	if err == nil && len(pods) == 1 {
		pod := pods[0]
		glog.V(2).Infof("Found existing vm pod %s/%s", pod.Namespace, pod.Name)
		// TODO check the pod against the current spec and update, if necessary
		if pod.DeletionTimestamp != nil {
			vm2.Status.State = vmapi.StateStopping
		}

		if common.IsPodReady(pod) {
			vm2.Status.State = vmapi.StateRunning
		}
	} else if err != nil && !apierrors.IsNotFound(err) {
		glog.V(2).Infof("error getting vm pod(s) %s/%s: %v", NamespaceVM, vm.Name, err)
		return
	} else if len(pods) > 1 {
		glog.V(2).Infof("more than one vm pod detected %s/%s: %v", NamespaceVM, vm.Name, pods)
		return
	} else {
		_, err = ctrl.kubeClient.CoreV1().Pods(NamespaceVM).Create(ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, false))
		if err != nil {
			glog.V(2).Infof("Error creating vm pod %s/%s: %v", NamespaceVM, vm.Name, err)
			return
		}
	}

	err = ctrl.updateVMStatus(vm, vm2)
	return
}

func (ctrl *VirtualMachineController) updateVMStatus(current *vmapi.VirtualMachine, updated *vmapi.VirtualMachine) (err error) {
	if !reflect.DeepEqual(current.Status, updated.Status) || !reflect.DeepEqual(current.Finalizers, updated.Finalizers) {
		updated, err = ctrl.vmClient.VirtualmachineV1alpha1().VirtualMachines().Update(updated)
	}
	return
}

func (ctrl *VirtualMachineController) startVM(vm *vmapi.VirtualMachine) (err error) {
	if err = ctrl.updateVmPod(vm); err != nil {
		glog.Warningf("error updating vm pod %s/%s: %v", NamespaceVM, vm.Name, err)
	}

	if err = ctrl.updateNovnc(vm); err != nil {
		glog.Warningf("error updating novnc %s/%s: %v", NamespaceVM, vm.Name, err)
	}

	return
}

func (ctrl *VirtualMachineController) stopVM(vm *vmapi.VirtualMachine) (err error) {
	vm2 := vm.DeepCopy()
	err = ctrl.deleteVmPod(NamespaceVM, vm.Name)
	switch {
	case err == nil:
		vm2.Status.State = vmapi.StateStopping
	case apierrors.IsNotFound(err):
		vm2.Status.State = vmapi.StateStopped
	default:
		vm2.Status.State = vmapi.StateError
	}

	err = ctrl.deleteNovncPod(NamespaceVM, vm.Name)
	switch {
	case err == nil:
		// if either the vm or novnc pod had to be deleted, we are stopping
		vm2.Status.State = vmapi.StateStopping
	case apierrors.IsNotFound(err):
		// if the novnc was already deleted, our state is dictated by the vm pod delete
	default:
		vm2.Status.State = vmapi.StateError
	}
	err = ctrl.updateVMStatus(vm, vm2)
	return
}

func (ctrl *VirtualMachineController) updateVM(vm *vmapi.VirtualMachine) {
	// set the instance id, mac address, finalizer if not present
	if vm.Status.ID == "" || vm.Status.MAC == "" || len(vm.Finalizers) == 0 {
		vm2 := vm.DeepCopy()
		uid := string(vm.UID)
		vm2.Finalizers = append(vm2.Finalizers, FinalizerDeletion)
		vm2.Status.ID = fmt.Sprintf("i-%s", uid[:8])
		vm2.Status.MAC = fmt.Sprintf("06:fe:%s:%s:%s:%s", uid[:2], uid[2:4], uid[4:6], uid[6:8])
		ctrl.updateVMStatus(vm, vm2)
		ctrl.updateVM(vm2)
		return
	}

	// TODO requeue if an error is returned by start/stop/migrate
	switch vm.Spec.Action {
	case vmapi.ActionStart:
		ctrl.startVM(vm)
	case vmapi.ActionStop:
		ctrl.stopVM(vm)
	case vmapi.ActionMigrate:
		ctrl.migrateVM(vm)
	default:
		glog.Warningf("detected vm %s/%s with invalid action \"%s\"", NamespaceVM, vm.Name, vm.Spec.Action)
		return
	}
}

func (ctrl *VirtualMachineController) deleteVmPod(ns, name string) error {
	glog.V(2).Infof("trying to delete pods associated with vm %s/%s", ns, name)

	vmPodSelector := labels.Set{
		"name": name,
	}.AsSelector()

	pods, _ := ctrl.podLister.Pods(NamespaceVM).List(vmPodSelector)
	if len(pods) == 0 {
		return apierrors.NewNotFound(corev1.Resource("pod"), name)
	}

	return ctrl.kubeClient.CoreV1().Pods(ns).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: vmPodSelector.String(),
	})
}

func (ctrl *VirtualMachineController) deleteVM(vm *vmapi.VirtualMachine) {

	// update status to terminating, if necessary
	if vm.Status.State != vmapi.StateTerminating {
		vm2 := vm.DeepCopy()
		vm2.Status.State = vmapi.StateTerminating
		ctrl.updateVMStatus(vm, vm2)
		vm = vm2
	}

	err1 := ctrl.deleteVmPod(NamespaceVM, vm.Name)
	glog.Infof("Delete VM pod error: %v", err1)
	err2 := ctrl.deleteNovncPod(NamespaceVM, vm.Name)
	glog.Infof("Delete novnc pod error: %v", err2)
	err3 := ctrl.deleteNovncService(NamespaceVM, vm.Name)
	glog.Infof("Delete novnc svc error: %v", err3)

	// TODO delete host path

	// Once dependent resources are all gone, remove finalizer and delete VM
	if apierrors.IsNotFound(err1) &&
		apierrors.IsNotFound(err2) &&
		apierrors.IsNotFound(err3) {

		vm2 := vm.DeepCopy()
		vm2.Finalizers = []string{}
		if err := ctrl.updateVMStatus(vm, vm2); err == nil {
			err = ctrl.vmClient.VirtualmachineV1alpha1().VirtualMachines().Delete(vm2.Name, &metav1.DeleteOptions{})
			// } else {
			// 	glog.V(5).Infof("requeued %q for sync", keyObj)
			// 	ctrl.vmQueue.Add(keyObj)
			// 	// requeue
		}
	}
}

func (ctrl *VirtualMachineController) vmWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.vmQueue.Get()
		if quit {
			return true
		}
		defer ctrl.vmQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("vmWorker[%s]", key)

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting name of vm %q to get vm from informer: %v", key, err)
			return false
		}
		vm, err := ctrl.vmLister.Get(name)
		switch {
		case err == nil:
			switch vm.DeletionTimestamp {
			case nil:
				ctrl.updateVM(vm)
			default:
				ctrl.deleteVM(vm)
			}
		case apierrors.IsNotFound(err):
			break
		default:
			glog.V(2).Infof("error getting vm %q from informer: %v", key, err)
			ctrl.vmQueue.Add(keyObj)
			glog.V(5).Infof("re-enqueued %q for sync", keyObj)
		}
		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("vm worker queue shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) podWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.podQueue.Get()
		if quit {
			return true
		}
		defer ctrl.podQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("podWorker[%s]", key)

		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting name of vm %q to get vm from informer: %v", key, err)
			return false
		}

		vmName := name[:strings.LastIndex(name, nameDelimiter)]

		_, err = ctrl.podLister.Pods(ns).Get(name)
		if err == nil {
			glog.V(5).Infof("enqueued vm %q for sync", vmName)
			ctrl.vmQueue.Add(vmName)
		} else if apierrors.IsNotFound(err) {
			glog.V(5).Infof("enqueued vm %q for sync", vmName)
			ctrl.vmQueue.Add(vmName)
		} else {
			glog.Warningf("error getting pod %q from informer: %v", key, err)
		}

		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("pod worker queue shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) podFilterFunc(obj interface{}) bool {
	if pod, ok := obj.(*corev1.Pod); ok {
		if app, ok := pod.Labels["app"]; ok && app == "ranchervm" {
			if role, ok := pod.Labels["role"]; ok && role == "vm" {
				return true
			}
		}
	}
	return false
}
