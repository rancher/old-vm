package server

import (
	"net/http"

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
}

func NewServer(
	vmClient vmclientset.Interface,
	kubeClient kubernetes.Interface,
	vmInformer vminformers.VirtualMachineInformer,
	nodeInformer coreinformers.NodeInformer,
	credInformer vminformers.CredentialInformer,
	listenAddress string,
) *server {

	return &server{
		vmClient:   vmClient,
		kubeClient: kubeClient,

		vmLister:         vmInformer.Lister(),
		vmListerSynced:   vmInformer.Informer().HasSynced,
		nodeLister:       nodeInformer.Lister(),
		nodeListerSynced: nodeInformer.Informer().HasSynced,
		credLister:       credInformer.Lister(),
		credListerSynced: credInformer.Informer().HasSynced,

		listenAddress: listenAddress,
	}
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

func (s *server) newRouter() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path("/v1/instances").Handler(http.HandlerFunc(s.InstanceList))
	r.Methods("POST").Path("/v1/instances").Handler(http.HandlerFunc(s.InstanceCreate))
	r.Methods("DELETE").Path("/v1/instances/{name}").Handler(http.HandlerFunc(s.InstanceDelete))
	r.Methods("POST").Path("/v1/instances/delete").Handler(http.HandlerFunc(s.InstanceDeleteMulti))
	r.Methods("POST").Path("/v1/instances/{name}/{action}").Handler(http.HandlerFunc(s.InstanceAction))
	r.Methods("POST").Path("/v1/instances/{action}").Handler(http.HandlerFunc(s.InstanceActionMulti))

	r.Methods("GET").Path("/v1/host").Handler(http.HandlerFunc(s.NodeList))

	r.Methods("GET").Path("/v1/credential").Handler(http.HandlerFunc(s.CredentialList))
	r.Methods("POST").Path("/v1/credential").Handler(http.HandlerFunc(s.CredentialCreate))
	r.Methods("DELETE").Path("/v1/credential/{name}").Handler(http.HandlerFunc(s.CredentialDelete))
	return r
}
