package vm

import (
	"math/rand"
	"os"
	"time"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	vmclientset "github.com/rancher/vm/pkg/client/clientset/versioned"
	vminformers "github.com/rancher/vm/pkg/client/informers/externalversions/ranchervm/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

type VirtualMachineController struct {
	vmClient   vmclientset.Interface
	kubeClient kubernetes.Interface

	podLister        corelisters.PodLister
	podListerSynced  cache.InformerSynced
	jobLister        batchlisters.JobLister
	jobListerSynced  cache.InformerSynced
	svcLister        corelisters.ServiceLister
	svcListerSynced  cache.InformerSynced
	pvLister         corelisters.PersistentVolumeLister
	pvListerSynced   cache.InformerSynced
	pvcLister        corelisters.PersistentVolumeClaimLister
	pvcListerSynced  cache.InformerSynced
	nodeLister       corelisters.NodeLister
	nodeListerSynced cache.InformerSynced

	machineLister            vmlisters.VirtualMachineLister
	machineListerSynced      cache.InformerSynced
	credLister               vmlisters.CredentialLister
	credListerSynced         cache.InformerSynced
	settingLister            vmlisters.SettingLister
	settingListerSynced      cache.InformerSynced
	machineImageLister       vmlisters.MachineImageLister
	machineImageListerSynced cache.InformerSynced

	podQueue          workqueue.RateLimitingInterface
	jobQueue          workqueue.RateLimitingInterface
	machineQueue      workqueue.RateLimitingInterface
	settingQueue      workqueue.RateLimitingInterface
	machineImageQueue workqueue.RateLimitingInterface

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
	nodeInformer coreinformers.NodeInformer,
	machineInformer vminformers.VirtualMachineInformer,
	credInformer vminformers.CredentialInformer,
	settingInformer vminformers.SettingInformer,
	machineImageInformer vminformers.MachineImageInformer,
	bridgeIface string,
	noResourceLimits bool,
) *VirtualMachineController {

	ctrl := &VirtualMachineController{
		vmClient:          vmClient,
		kubeClient:        kubeClient,
		machineQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		podQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		jobQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "job"),
		settingQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "setting"),
		machineImageQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineimage"),
		bridgeIface:       bridgeIface,
		noResourceLimits:  noResourceLimits,
	}

	machineInformer.Informer().AddEventHandler(ctrl.queueEventHandler(ctrl.machineQueue))
	settingInformer.Informer().AddEventHandler(ctrl.queueEventHandler(ctrl.settingQueue))
	machineImageInformer.Informer().AddEventHandler(ctrl.queueEventHandler(ctrl.machineImageQueue))
	podInformer.Informer().AddEventHandler(ctrl.podEventHandler())
	jobInformer.Informer().AddEventHandler(ctrl.jobEventHandler())
	nodeInformer.Informer().AddEventHandler(ctrl.nodeEventHandler())

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

	ctrl.nodeLister = nodeInformer.Lister()
	ctrl.nodeListerSynced = nodeInformer.Informer().HasSynced

	ctrl.machineLister = machineInformer.Lister()
	ctrl.machineListerSynced = machineInformer.Informer().HasSynced

	ctrl.credLister = credInformer.Lister()
	ctrl.credListerSynced = credInformer.Informer().HasSynced

	ctrl.settingLister = settingInformer.Lister()
	ctrl.settingListerSynced = settingInformer.Informer().HasSynced

	ctrl.machineImageLister = machineImageInformer.Lister()
	ctrl.machineImageListerSynced = machineImageInformer.Informer().HasSynced

	return ctrl
}

func (ctrl *VirtualMachineController) queueEventHandler(queue workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { ctrl.enqueueWork(queue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(queue, newObj) },
		DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(queue, obj) },
	}
}

func (ctrl *VirtualMachineController) podEventHandler() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			if pod, ok := obj.(*corev1.Pod); ok {
				if app, ok := pod.Labels["app"]; ok && app == common.LabelApp {
					// look at job events instead for migration pod events
					if role, ok := pod.Labels["role"]; ok && role != common.LabelRoleMigrate {
						return true
					}
				}
			}
			return false
		},
		Handler: ctrl.queueEventHandler(ctrl.podQueue),
	}
}

func (ctrl *VirtualMachineController) jobEventHandler() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			if job, ok := obj.(*batchv1.Job); ok {
				if app, ok := job.Labels["app"]; ok && app == common.LabelApp {
					return true
				}
			}
			return false
		},
		Handler: ctrl.queueEventHandler(ctrl.jobQueue),
	}
}

func (ctrl *VirtualMachineController) nodeEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// perform a full resync on MachineImage
			machineImages, err := ctrl.machineImageLister.List(labels.Everything())
			if err != nil {
				glog.Warningf("Couldn't list machine images: %v", err)
				return
			}
			for _, machineImage := range machineImages {
				ctrl.enqueueWork(ctrl.machineImageQueue, machineImage)
			}
		},
	}
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

func HostnameOrDie() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return hostname
}

func (ctrl *VirtualMachineController) run(stopCh <-chan struct{}) {
	defer ctrl.podQueue.ShutDown()
	defer ctrl.jobQueue.ShutDown()
	defer ctrl.machineQueue.ShutDown()
	defer ctrl.settingQueue.ShutDown()
	defer ctrl.machineImageQueue.ShutDown()

	glog.Infof("starting vm controller")
	defer glog.Infof("stopping vm controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.podListerSynced,
		ctrl.jobListerSynced, ctrl.svcListerSynced,
		ctrl.pvListerSynced, ctrl.pvcListerSynced,
		ctrl.machineListerSynced, ctrl.credListerSynced,
		ctrl.settingListerSynced, ctrl.machineImageListerSynced,
		ctrl.nodeListerSynced) {
		return
	}

	if err := ctrl.initializeSettings(); err != nil {
		glog.Warningf(err.Error())
	}
	if err := ctrl.updateLonghornClient(); err != nil {
		glog.Warningf(err.Error())
	}

	go wait.Until(ctrl.podWorker, time.Second, stopCh)
	go wait.Until(ctrl.jobWorker, time.Second, stopCh)
	go wait.Until(ctrl.machineWorker, time.Second, stopCh)
	go wait.Until(ctrl.settingWorker, time.Second, stopCh)
	go wait.Until(ctrl.machineImageWorker, time.Second, stopCh)

	<-stopCh
}
