package vm

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	vmclientset "github.com/rancher/vm/pkg/client/clientset/versioned"
	vminformers "github.com/rancher/vm/pkg/client/informers/externalversions/ranchervm/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

type VirtualMachineController struct {
	vmClient   vmclientset.Interface
	kubeClient kubernetes.Interface

	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced
	jobLister       batchlisters.JobLister
	jobListerSynced cache.InformerSynced
	svcLister       corelisters.ServiceLister
	svcListerSynced cache.InformerSynced
	pvLister        corelisters.PersistentVolumeLister
	pvListerSynced  cache.InformerSynced
	pvcLister       corelisters.PersistentVolumeClaimLister
	pvcListerSynced cache.InformerSynced

	vmLister            vmlisters.VirtualMachineLister
	vmListerSynced      cache.InformerSynced
	credLister          vmlisters.CredentialLister
	credListerSynced    cache.InformerSynced
	settingLister       vmlisters.SettingLister
	settingListerSynced cache.InformerSynced

	podQueue     workqueue.RateLimitingInterface
	jobQueue     workqueue.RateLimitingInterface
	vmQueue      workqueue.RateLimitingInterface
	settingQueue workqueue.RateLimitingInterface

	bridgeIface      string
	noResourceLimits bool

	lhClient *LonghornClient
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewVirtualMachineController(
	vmClient vmclientset.Interface,
	kubeClient kubernetes.Interface,
	podInformer coreinformers.PodInformer,
	jobInformer batchinformers.JobInformer,
	svcInformer coreinformers.ServiceInformer,
	pvInformer coreinformers.PersistentVolumeInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	vmInformer vminformers.VirtualMachineInformer,
	credInformer vminformers.CredentialInformer,
	settingInformer vminformers.SettingInformer,
	bridgeIface string,
	noResourceLimits bool,
) *VirtualMachineController {

	ctrl := &VirtualMachineController{
		vmClient:         vmClient,
		kubeClient:       kubeClient,
		vmQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "virtualmachine"),
		podQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		jobQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "job"),
		settingQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "setting"),
		bridgeIface:      bridgeIface,
		noResourceLimits: noResourceLimits,
	}

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

	jobInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: ctrl.jobFilterFunc,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.jobQueue, obj) },
				UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.jobQueue, newObj) },
				DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.jobQueue, obj) },
			},
		},
	)

	vmInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.vmQueue, obj) },
		},
	)

	settingInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.settingQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.settingQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.settingQueue, obj) },
		},
	)

	ctrl.podLister = podInformer.Lister()
	ctrl.podListerSynced = podInformer.Informer().HasSynced

	ctrl.jobLister = jobInformer.Lister()
	ctrl.jobListerSynced = jobInformer.Informer().HasSynced

	ctrl.svcLister = svcInformer.Lister()
	ctrl.svcListerSynced = svcInformer.Informer().HasSynced

	ctrl.pvLister = pvInformer.Lister()
	ctrl.pvListerSynced = pvInformer.Informer().HasSynced

	ctrl.pvcLister = pvcInformer.Lister()
	ctrl.pvcListerSynced = pvcInformer.Informer().HasSynced

	ctrl.vmLister = vmInformer.Lister()
	ctrl.vmListerSynced = vmInformer.Informer().HasSynced

	ctrl.credLister = credInformer.Lister()
	ctrl.credListerSynced = credInformer.Informer().HasSynced

	ctrl.settingLister = settingInformer.Lister()
	ctrl.settingListerSynced = settingInformer.Informer().HasSynced

	return ctrl
}

func HostnameOrDie() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return hostname
}

func (ctrl *VirtualMachineController) Run() {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(ctrl.kubeClient.Core().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(runtime.NewScheme(), corev1.EventSource{Component: "ranchervm-controller"})

	endpointLock, err := resourcelock.New(
		resourcelock.EndpointsResourceLock,
		"ranchervm-system",
		"ranchervm-controller",
		ctrl.kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      HostnameOrDie(),
			EventRecorder: recorder,
		},
	)
	if err != nil {
		panic(err)
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          endpointLock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(stop <-chan struct{}) {
				glog.Info("started leading")
				ctrl.run(stop)
			},
			OnStoppedLeading: func() {
				glog.Info("stopped leading")
			},
			OnNewLeader: func(identity string) {
				glog.Infof("new leader: %s", identity)
			},
		},
	})
}

func (ctrl *VirtualMachineController) run(stopCh <-chan struct{}) {
	defer ctrl.vmQueue.ShutDown()
	defer ctrl.podQueue.ShutDown()
	defer ctrl.jobQueue.ShutDown()
	defer ctrl.settingQueue.ShutDown()

	glog.Infof("starting vm controller")
	defer glog.Infof("stopping vm controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.podListerSynced,
		ctrl.jobListerSynced, ctrl.svcListerSynced,
		ctrl.pvListerSynced, ctrl.pvcListerSynced,
		ctrl.vmListerSynced, ctrl.credListerSynced,
		ctrl.settingListerSynced) {
		return
	}

	if err := ctrl.initSettings(); err != nil {
		glog.Warningf(err.Error())
	}
	if err := ctrl.updateLonghornClient(); err != nil {
		glog.Warningf(err.Error())
	}

	go wait.Until(ctrl.vmWorker, time.Second, stopCh)
	go wait.Until(ctrl.podWorker, time.Second, stopCh)
	go wait.Until(ctrl.jobWorker, time.Second, stopCh)
	go wait.Until(ctrl.settingWorker, time.Second, stopCh)

	<-stopCh
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
		if err != nil {
			if !apierrors.IsNotFound(err) {
				glog.V(2).Infof("error getting vm %q from informer: %v", key, err)
				ctrl.vmQueue.AddRateLimited(keyObj)
				glog.V(5).Infof("re-enqueued %q for sync", keyObj)
			}
			return false
		}

		if err := ctrl.process(vm, keyObj); err != nil {
			ctrl.vmQueue.AddRateLimited(keyObj)
			glog.V(5).Infof("error %v, re-enqueued %q for sync", err, keyObj)
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
			glog.V(4).Infof("error getting name of vm %q: %v", key, err)
			return false
		}

		vmName := name[:strings.LastIndex(name, common.NameDelimiter)]

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
		if app, ok := pod.Labels["app"]; ok && app == common.LabelApp {
			// look at job events instead for migration pod events
			if role, ok := pod.Labels["role"]; ok && role != common.LabelRoleMigrate {
				return true
			}
		}
	}
	return false
}

func (ctrl *VirtualMachineController) jobWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.jobQueue.Get()
		if quit {
			return true
		}
		defer ctrl.jobQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("jobWorker[%s]", key)

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting name of vm %q: %v", key, err)
			return false
		}

		vmName := name[:strings.LastIndex(name, common.NameDelimiter)]
		ctrl.vmQueue.Add(vmName)
		glog.V(5).Infof("enqueued vm %q for sync", vmName)

		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("job worker queue shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) jobFilterFunc(obj interface{}) bool {
	if job, ok := obj.(*batchv1.Job); ok {
		if app, ok := job.Labels["app"]; ok && app == common.LabelApp {
			return true
		}
	}
	return false
}

func (ctrl *VirtualMachineController) process(vm *vmapi.VirtualMachine, keyObj interface{}) error {
	if vm.DeletionTimestamp == nil {
		// set the instance id, mac address, finalizer if not present
		if vm.Status.ID == "" || vm.Status.MAC == "" || len(vm.Finalizers) == 0 {
			vm2 := vm.DeepCopy()
			uid := string(vm.UID)
			vm2.Finalizers = append(vm2.Finalizers, common.FinalizerDeletion)
			vm2.Status.ID = fmt.Sprintf("i-%s", uid[:8])
			vm2.Status.MAC = fmt.Sprintf("%s:%s:%s:%s:%s", common.RancherOUI, uid[:2], uid[2:4], uid[4:6], uid[6:8])
			return ctrl.updateVMStatus(vm, vm2)
		}

		var err error
		switch vm.Spec.Action {
		case vmapi.ActionStart:
			if vm.Spec.Volume.Longhorn != nil {
				if err := ctrl.createLonghornVolume(vm); err != nil {
					return err
				}
			}
			err = ctrl.start(vm)
		case vmapi.ActionStop:
			err = ctrl.stop(vm)
		default:
			glog.Warningf("detected vm %s/%s with invalid action \"%s\"", common.NamespaceVM, vm.Name, vm.Spec.Action)
			// TODO change VM state to ERROR, return no error (don't requeue)
			return nil
		}
		return err
	} else {
		if vm.Status.State != vmapi.StateTerminating {
			return ctrl.setTerminating(vm)
		}

		err1 := ctrl.deleteMachinePods(vm)
		err2 := ctrl.deleteConsolePod(vm)
		err3 := ctrl.deleteConsoleService(vm)

		if vm.Spec.Volume.Longhorn != nil {
			if err := ctrl.deleteLonghornVolume(vm); err != nil {
				return err
			}
		}
		// TODO delete host path (or emptyDir refactor)

		if apierrors.IsNotFound(err1) &&
			apierrors.IsNotFound(err2) &&
			apierrors.IsNotFound(err3) {

			return ctrl.removeFinalizer(vm)
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) createLonghornVolume(vm *vmapi.VirtualMachine) error {
	if vol, err := ctrl.lhClient.GetVolume(vm.Name); err != nil {
		return err
	} else if vol == nil {
		if err := ctrl.lhClient.CreateVolume(vm); err != nil {
			return err
		}
	}

	if _, err := ctrl.pvLister.Get(vm.Name); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := ctrl.createPersistentVolume(vm); err != nil {
			return err
		}
	}

	if _, err := ctrl.pvcLister.PersistentVolumeClaims(common.NamespaceVM).Get(vm.Name); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := ctrl.createPersistentVolumeClaim(vm); err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) deleteLonghornVolume(vm *vmapi.VirtualMachine) error {
	if vol, err := ctrl.lhClient.GetVolume(vm.Name); err != nil {
		return err
	} else if vol != nil {
		if err := ctrl.lhClient.DeleteVolume(vm.Name); err != nil {
			return err
		}
	}

	if err := ctrl.kubeClient.CoreV1().PersistentVolumes().Delete(vm.Name, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(common.NamespaceVM).Delete(vm.Name, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) createPersistentVolume(vm *vmapi.VirtualMachine) error {
	_, err := ctrl.kubeClient.CoreV1().PersistentVolumes().Create(&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: vm.Name,
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Capacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceStorage: resource.MustParse(vm.Spec.Volume.Longhorn.Size),
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "io.rancher.longhorn",
					VolumeAttributes: map[string]string{
						"frontend": "iscsi",
					},
					VolumeHandle: vm.Name,
				},
			},
		},
	})
	return err
}

var noStorageClass = ""

func (ctrl *VirtualMachineController) createPersistentVolumeClaim(vm *vmapi.VirtualMachine) error {
	_, err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(common.NamespaceVM).Create(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: vm.Name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse(vm.Spec.Volume.Longhorn.Size),
				},
			},
			StorageClassName: &noStorageClass,
			VolumeName:       vm.Name,
		},
	})
	return err
}

func (ctrl *VirtualMachineController) setTerminating(vm *vmapi.VirtualMachine) error {
	vm2 := vm.DeepCopy()
	vm2.Status.State = vmapi.StateTerminating
	return ctrl.updateVMStatus(vm, vm2)
}

func (ctrl *VirtualMachineController) removeFinalizer(vm *vmapi.VirtualMachine) error {
	vm2 := vm.DeepCopy()
	vm2.Finalizers = []string{}
	return ctrl.updateVMStatus(vm, vm2)
}

func (ctrl *VirtualMachineController) initSettings() error {
	for name, definition := range vmapi.SettingDefinitions {
		setting, err := ctrl.settingLister.Get(string(name))
		if apierrors.IsNotFound(err) || setting == nil {
			setting := &vmapi.Setting{
				// I shouldn't have to set the type meta, what's wrong with the client?
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vm.rancher.io/v1alpha1",
					Kind:       "Setting",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: string(name),
				},
				Spec: vmapi.SettingSpec{
					Value: definition.Default,
				},
			}
			if _, err := ctrl.vmClient.VirtualmachineV1alpha1().Settings().Create(setting); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
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
	queue.Add(objName)
}

func (ctrl *VirtualMachineController) updateVMPod(vm *vmapi.VirtualMachine) (vm2 *vmapi.VirtualMachine, pod *corev1.Pod, err error) {
	vm2 = vm
	pods, err := ctrl.podLister.Pods(common.NamespaceVM).List(labels.Set{
		"app":  common.LabelApp,
		"name": vm.Name,
		"role": common.LabelRoleVM,
	}.AsSelector())

	if err != nil && !apierrors.IsNotFound(err) {
		glog.V(2).Infof("Error getting vm pod(s) %s/%s: %v", common.NamespaceVM, vm.Name, err)
		return
	}

	alivePods := GetAlivePods(pods)
	switch len(alivePods) {
	case 0:
		if vm.Spec.Volume.Longhorn != nil {
			pod = ctrl.createLonghornMachinePod(vm, false)
		} else {
			pod = ctrl.makeVMPod(vm, ctrl.bridgeIface, ctrl.noResourceLimits, false)
		}
		pod, err = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod)
		if err != nil {
			glog.V(2).Infof("Error creating vm pod %s/%s: %v", common.NamespaceVM, vm.Name, err)
			return
		}
	case 1:
		pod = alivePods[0]
	default:
		return
	}

	vm2 = vm.DeepCopy()
	err = ctrl.updateVMStatusWithPod(vm, vm2, pod)
	return
}

func (ctrl *VirtualMachineController) updateVMStatusWithPod(vm *vmapi.VirtualMachine, vm2 *vmapi.VirtualMachine, pod *corev1.Pod) error {
	if pod.Spec.NodeName != "" {
		vm2.Status.NodeName = pod.Spec.NodeName
	}
	if pod.Status.HostIP != "" {
		vm2.Status.NodeIP = pod.Status.HostIP
	}
	switch {
	case pod.DeletionTimestamp != nil:
		vm2.Status.State = vmapi.StateStopping
	case common.IsPodReady(pod):
		vm2.Status.State = vmapi.StateRunning
	default:
		vm2.Status.State = vmapi.StatePending
	}
	return ctrl.updateVMStatus(vm, vm2)
}

func (ctrl *VirtualMachineController) updateVMStatus(current *vmapi.VirtualMachine, updated *vmapi.VirtualMachine) (err error) {
	if !reflect.DeepEqual(current.Status, updated.Status) ||
		!reflect.DeepEqual(current.Finalizers, updated.Finalizers) ||
		!reflect.DeepEqual(current.Spec, updated.Spec) {
		updated, err = ctrl.vmClient.VirtualmachineV1alpha1().VirtualMachines().Update(updated)
	}
	return
}

func (ctrl *VirtualMachineController) start(vm *vmapi.VirtualMachine) error {
	vm, pod, err := ctrl.updateVMPod(vm)
	if err != nil {
		glog.Warningf("error updating vm pod %s/%s: %v", common.NamespaceVM, vm.Name, err)
		return err
	}

	if pod != nil && pod.Name != "" {
		if err = ctrl.updateNovnc(vm, pod.Name); err != nil {
			glog.Warningf("error updating novnc %s/%s: %v", common.NamespaceVM, vm.Name, err)
		}
	}

	// If vm is in pending state and pod is unschedulable, check to see if the
	// requested node name matches the pod node affinity. If they are mismatched,
	// delete the pod and allow the process to start over.
	if vm.Status.State == vmapi.StatePending && pod != nil && IsPodUnschedulable(pod) {
		if pod.Spec.Affinity != nil &&
			pod.Spec.Affinity.NodeAffinity != nil &&
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
			len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 1 {

			nodeSelector := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			if len(nodeSelector.NodeSelectorTerms) == 1 &&
				len(nodeSelector.NodeSelectorTerms[0].MatchExpressions) == 1 {

				requirement := nodeSelector.NodeSelectorTerms[0].MatchExpressions[0]
				if requirement.Key == common.LabelNodeHostname &&
					requirement.Operator == corev1.NodeSelectorOpIn &&
					len(requirement.Values) == 1 &&
					vm.Spec.NodeName != requirement.Values[0] {

					glog.V(2).Infof("User modified selector for unschedulable vm %s", vm.Name)
					return ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(pod.Name, &metav1.DeleteOptions{})
				}
			}
		}
	}

	if pod != nil && vm.Spec.NodeName != "" &&
		vm.Spec.NodeName != pod.Spec.NodeName &&
		vm.Status.State == vmapi.StateRunning ||
		vm.Status.State == vmapi.StateMigrating {
		return ctrl.migrateVM(vm)
	}

	return err
}

func (ctrl *VirtualMachineController) stop(vm *vmapi.VirtualMachine) (err error) {
	vm2 := vm.DeepCopy()

	if err = ctrl.deleteMachinePods(vm); err == nil {
		vm2.Status.State = vmapi.StateStopping
	} else if apierrors.IsNotFound(err) {
		vm2.Status.State = vmapi.StateStopped
		vm2.Status.NodeName = ""
	} else {
		vm2.Status.State = vmapi.StateError
	}

	if err = ctrl.deleteMigrationJob(vm); err == nil {
		vm2.Status.State = vmapi.StateStopping
	} else if !apierrors.IsNotFound(err) {
		vm2.Status.State = vmapi.StateError
	}

	if err = ctrl.deleteConsolePod(vm); err == nil {
		// if either the vm or novnc pod had to be deleted, we are stopping
		vm2.Status.State = vmapi.StateStopping
	} else if !apierrors.IsNotFound(err) {
		vm2.Status.State = vmapi.StateError
	}

	err = ctrl.updateVMStatus(vm, vm2)
	return
}

func (ctrl *VirtualMachineController) deleteMachinePods(vm *vmapi.VirtualMachine) error {
	vmPodSelector := labels.Set{"name": vm.Name}.AsSelector()

	pods, _ := ctrl.podLister.Pods(common.NamespaceVM).List(vmPodSelector)
	if len(pods) == 0 {
		return apierrors.NewNotFound(corev1.Resource("pods"), vm.Name+"-*")
	}

	return ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: vmPodSelector.String()})
}
