package server

import (
	"net/http"
	"sync"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	vmclientset "github.com/rancher/vm/pkg/client/clientset/versioned"
	vminformers "github.com/rancher/vm/pkg/client/informers/externalversions/virtualmachine/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/virtualmachine/v1alpha1"
)

type SimpleResourceEventHandler struct{ ChangeFunc func() }

func (s SimpleResourceEventHandler) OnAdd(obj interface{})               { s.ChangeFunc() }
func (s SimpleResourceEventHandler) OnUpdate(oldObj, newObj interface{}) { s.ChangeFunc() }
func (s SimpleResourceEventHandler) OnDelete(obj interface{})            { s.ChangeFunc() }

type Watcher struct {
	eventChan chan struct{}
	resources []string
	server    *server
}

func (w *Watcher) Events() <-chan struct{} {
	return w.eventChan
}

func (w *Watcher) Close() {
	s := w.server
	s.watcherLock.Lock()
	defer s.watcherLock.Unlock()

	close(w.eventChan)
	for i, watcher := range s.watchers {
		if watcher == w {
			s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
			return
		}
	}
	glog.Warningf("failed to remove from watchers: %+v", w)
}

type server struct {
	vmClient   vmclientset.Interface
	kubeClient kubernetes.Interface

	vmLister         vmlisters.VirtualMachineLister
	vmListerSynced   cache.InformerSynced
	nodeLister       corelisters.NodeLister
	nodeListerSynced cache.InformerSynced
	credLister       vmlisters.CredentialLister
	credListerSynced cache.InformerSynced

	listenAddress string
	watchers      []*Watcher
	watcherLock   sync.Mutex
}

func NewServer(
	vmClient vmclientset.Interface,
	kubeClient kubernetes.Interface,
	vmInformer vminformers.VirtualMachineInformer,
	nodeInformer coreinformers.NodeInformer,
	credInformer vminformers.CredentialInformer,
	listenAddress string,
) *server {

	s := &server{
		vmClient:   vmClient,
		kubeClient: kubeClient,

		vmLister:         vmInformer.Lister(),
		vmListerSynced:   vmInformer.Informer().HasSynced,
		nodeLister:       nodeInformer.Lister(),
		nodeListerSynced: nodeInformer.Informer().HasSynced,
		credLister:       credInformer.Lister(),
		credListerSynced: credInformer.Informer().HasSynced,

		listenAddress: listenAddress,
		watchers:      []*Watcher{},
	}

	vmInformer.Informer().AddEventHandler(s.notifyWatchersHandler("virtualmachine"))
	nodeInformer.Informer().AddEventHandler(s.notifyWatchersHandler("node"))
	credInformer.Informer().AddEventHandler(s.notifyWatchersHandler("credential"))

	return s
}

func (s *server) Run(stopCh <-chan struct{}) {
	if !cache.WaitForCacheSync(stopCh, s.vmListerSynced, s.nodeListerSynced, s.credListerSynced) {
		return
	}

	r := s.newRouter()
	glog.Infof("Starting http server listening on %s", s.listenAddress)
	go http.ListenAndServe(s.listenAddress, r)

	<-stopCh
}

func (s *server) NewWatcher(resources ...string) *Watcher {
	s.watcherLock.Lock()
	defer s.watcherLock.Unlock()

	w := &Watcher{
		eventChan: make(chan struct{}, 2),
		resources: resources,
		server:    s,
	}
	s.watchers = append(s.watchers, w)
	return w
}

func (s *server) notifyWatchersHandler(resource string) cache.ResourceEventHandler {
	return SimpleResourceEventHandler{
		ChangeFunc: s.notifyWatchersFunc(resource),
	}
}

func (s *server) notifyWatchersFunc(resource string) func() {
	return func() {
		s.watcherLock.Lock()
		defer s.watcherLock.Unlock()
		for _, w := range s.watchers {
			for _, r := range w.resources {
				if r == resource {
					select {
					case w.eventChan <- struct{}{}:
					default:
					}
					break
				}
			}
		}
	}
}

func (s *server) newRouter() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path("/v1/instances").Handler(http.HandlerFunc(s.InstanceList))
	r.Methods("POST").Path("/v1/instances").Handler(http.HandlerFunc(s.InstanceCreate))
	r.Methods("PUT").Path("/v1/instances").Handler(http.HandlerFunc(s.InstanceUpdate))
	r.Methods("GET").Path("/v1/instances/{name}").Handler(http.HandlerFunc(s.InstanceGet))
	r.Methods("DELETE").Path("/v1/instances/{name}").Handler(http.HandlerFunc(s.InstanceDelete))
	r.Methods("POST").Path("/v1/instances/delete").Handler(http.HandlerFunc(s.InstanceDeleteMulti))
	r.Methods("POST").Path("/v1/instances/{name}/{action}").Handler(http.HandlerFunc(s.InstanceAction))
	r.Methods("POST").Path("/v1/instances/{action}").Handler(http.HandlerFunc(s.InstanceActionMulti))

	r.Methods("GET").Path("/v1/host").Handler(http.HandlerFunc(s.NodeList))

	r.Methods("GET").Path("/v1/credential").Handler(http.HandlerFunc(s.CredentialList))
	r.Methods("POST").Path("/v1/credential").Handler(http.HandlerFunc(s.CredentialCreate))
	r.Methods("GET").Path("/v1/credential/{name}").Handler(http.HandlerFunc(s.CredentialGet))
	r.Methods("DELETE").Path("/v1/credential/{name}").Handler(http.HandlerFunc(s.CredentialDelete))

	instanceWatcher := s.NewWatcher("virtualmachine")
	defer instanceWatcher.Close()
	instanceListStream := NewStreamHandlerFunc(instanceWatcher, s.instanceList)
	r.Path("/v1/ws/instances").Handler(http.HandlerFunc(instanceListStream))
	r.Path("/v1/ws/{period}/instances").Handler(http.HandlerFunc(instanceListStream))

	nodeWatcher := s.NewWatcher("node")
	defer nodeWatcher.Close()
	nodeListStream := NewStreamHandlerFunc(nodeWatcher, s.nodeList)
	r.Path("/v1/ws/host").Handler(http.HandlerFunc(nodeListStream))
	r.Path("/v1/ws/{period}/host").Handler(http.HandlerFunc(nodeListStream))

	credentialWatcher := s.NewWatcher("credential")
	defer credentialWatcher.Close()
	credentialListStream := NewStreamHandlerFunc(credentialWatcher, s.credentialList)
	r.Path("/v1/ws/credential").Handler(http.HandlerFunc(credentialListStream))
	r.Path("/v1/ws/{period}/credential").Handler(http.HandlerFunc(credentialListStream))

	return r
}
