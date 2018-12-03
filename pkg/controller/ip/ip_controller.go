package ip

import (
	"bufio"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	vmclientset "github.com/rancher/vm/pkg/client/clientset/versioned"
	vminformers "github.com/rancher/vm/pkg/client/informers/externalversions/ranchervm/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

type IPDiscoveryController struct {
	crdClient       vmclientset.Interface
	arpLister       vmlisters.ARPTableLister
	arpListerSynced cache.InformerSynced
	vmLister        vmlisters.VirtualMachineLister
	vmListerSynced  cache.InformerSynced
	arpQueue        workqueue.RateLimitingInterface
	deviceName      string
}

func NewIPDiscoveryController(
	crdClient vmclientset.Interface,
	arpInformer vminformers.ARPTableInformer,
	vmInformer vminformers.VirtualMachineInformer,
	deviceName string,
) *IPDiscoveryController {

	ctrl := &IPDiscoveryController{
		crdClient:       crdClient,
		arpLister:       arpInformer.Lister(),
		arpListerSynced: arpInformer.Informer().HasSynced,
		vmLister:        vmInformer.Lister(),
		vmListerSynced:  vmInformer.Informer().HasSynced,
		arpQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "virtualmachine"),
		deviceName:      deviceName,
	}

	arpInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.arpQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.arpQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.arpQueue, obj) },
		},
	)

	return ctrl
}

func (ctrl *IPDiscoveryController) Run(workers int, stopCh <-chan struct{}) {
	defer ctrl.arpQueue.ShutDown()

	glog.Infof("Starting ip discovery controller")
	defer glog.Infof("Shutting down ip discovery Controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.arpListerSynced, ctrl.vmListerSynced) {
		return
	}

	go wait.Until(ctrl.arpWorker, time.Second, stopCh)
	go periodically(5*time.Second, ctrl.updateARPTable)

	<-stopCh
}

func periodically(t time.Duration, f func()) {
	c := time.Tick(t)
	for _ = range c {
		f()
	}
}

func (ctrl *IPDiscoveryController) updateMachines(arpTable *vmapi.ARPTable) error {
	vms, err := ctrl.vmLister.List(labels.Everything())
	if err != nil {
		return err
	}

	arpMap := arpTable.Spec.Table
	for _, vm := range vms {
		// ip resolution requires a mac address
		if vm.Status.MAC == "" {
			continue
		}

		if vm.Status.IP == "" {
			if entry, ok := arpMap[vm.Status.MAC]; ok {
				vm2 := vm.DeepCopy()
				vm2.Status.IP = entry.IP
				ctrl.updateMachineStatus(vm, vm2)
			}
		} else {
			if entry, ok := arpMap[vm.Status.MAC]; ok && entry.IP != vm.Status.IP {
				vm2 := vm.DeepCopy()
				vm2.Status.IP = entry.IP
				ctrl.updateMachineStatus(vm, vm2)
			}
		}
	}

	return nil
}

func (ctrl *IPDiscoveryController) updateMachineStatus(current *vmapi.VirtualMachine, updated *vmapi.VirtualMachine) (err error) {
	if !reflect.DeepEqual(current.Status, updated.Status) {
		updated, err = ctrl.crdClient.VirtualmachineV1alpha1().VirtualMachines().Update(updated)
		glog.V(3).Infof("Updated vm %s", updated.Name)
	}
	return
}

func (ctrl *IPDiscoveryController) updateARPTable() {
	newTable := &vmapi.ARPTable{
		// I shouldn't have to set the type meta, what's wrong with the client?
		TypeMeta: metav1.TypeMeta{
			APIVersion: "vm.rancher.io/v1alpha1",
			Kind:       "ARPTable",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ctrl.deviceName,
		},
		Spec: vmapi.ARPTableSpec{
			Table: map[string]vmapi.ARPEntry{},
		},
	}

	curTable, err := ctrl.arpLister.Get(ctrl.deviceName)
	if err == nil {
		newTable = curTable.DeepCopy()
	} else if !apierrors.IsNotFound(err) {
		glog.V(2).Infof("error getting arptable for device %s: %v", ctrl.deviceName, err)
		return
	}

	arpHandle, err := os.Open("/proc/net/arp")
	if err != nil {
		glog.Warningf(err.Error())
		return
	}
	defer arpHandle.Close()

	arp := bufio.NewScanner(arpHandle)
	for arp.Scan() {
		l := arp.Text()
		// ignore header
		if strings.HasPrefix(l, "IP") {
			continue
		}
		f := strings.Fields(l)
		// ignore invalid entries
		if len(f) != 6 {
			continue
		}
		// only store entries on the managed bridge
		// if f[5] != "br0" {
		// 	continue
		// }
		// only store entries involving rancher vms
		if !strings.HasPrefix(f[3], common.RancherOUI) {
			continue
		}

		newTable.Spec.Table[f[3]] = vmapi.ARPEntry{
			IP:        f[0],
			HWType:    f[1],
			Flags:     f[2],
			HWAddress: f[3],
			Mask:      f[4],
			Device:    f[5],
		}
	}

	if curTable == nil {
		newTable, err = ctrl.crdClient.VirtualmachineV1alpha1().ARPTables().Create(newTable)
	} else {
		if !reflect.DeepEqual(curTable.Spec.Table, newTable.Spec.Table) {
			newTable, err = ctrl.crdClient.VirtualmachineV1alpha1().ARPTables().Update(newTable)
		}
	}
	if err != nil {
		glog.Warningf(err.Error())
	}
}

func (ctrl *IPDiscoveryController) enqueueWork(queue workqueue.Interface, obj interface{}) {
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

func (ctrl *IPDiscoveryController) arpWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.arpQueue.Get()
		if quit {
			return true
		}
		defer ctrl.arpQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("arpWorker[%s]", key)

		_, deviceName, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting name of arptable %q to get arptable from informer: %v", key, err)
			return false
		}
		arpMap, err := ctrl.arpLister.Get(deviceName)
		if err == nil {
			ctrl.updateMachines(arpMap)
			return false
		}
		if !apierrors.IsNotFound(err) {
			glog.V(2).Infof("error getting arptable %q from informer: %v", key, err)
			return false
		}

		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("arp worker queue shutting down")
			return
		}
	}
}
