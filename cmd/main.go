package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/rancher/vm/pkg/client/clientset/versioned"
	"github.com/rancher/vm/pkg/client/informers/externalversions"
	"github.com/rancher/vm/pkg/controller/ip"
	"github.com/rancher/vm/pkg/controller/vm"
	"github.com/rancher/vm/pkg/qemu"
	"github.com/rancher/vm/pkg/server"
)

func main() {
	// common flags
	kubeconfig := flag.String("kubeconfig", "", "Path to a kube config; only required if out-of-cluster.")
	workers := flag.Int("workers", 5, "Concurrent resource syncs")
	flag.Set("logtostderr", "true")

	// vm-controller flags
	vmCtrl := flag.Bool("vm", false, "Run the VM controller")
	bridgeIface := flag.String("bridge-iface", "ens33", "Target network interface to bridge VM network to")
	noResourceLimits := flag.Bool("no-resource-limits", false, "Disable resource limits (proceed at your own risk)")

	// ip-controller flags
	ipCtrl := flag.Bool("ip", false, "Run the IP controller")
	deviceName := flag.String("devicename", "br0", "Name of the device to store ARP entries for")

	// rest-server flags
	serv := flag.Bool("backend", false, "Run the REST server backend")
	listenAddress := flag.String("listen-address", ":9500", "TCP network address that the REST server will listen on")

	// migration flags
	migrate := flag.Bool("migrate", false, "Run the VM migration job")
	sockPath := flag.String("sock-path", "", "Path to VM monitor Unix domain socket")
	targetURI := flag.String("target-uri", "", "URI of the target VM to migrate to")

	flag.Parse()

	if *migrate {
		c := qemu.NewMonitorClient(*sockPath)

		if err := c.Migrate(*targetURI); err != nil {
			panic(err)
		}
		return
	}

	config, err := NewKubeClientConfig(*kubeconfig)
	if err != nil {
		panic(err)
	}

	vmClientset := versioned.NewForConfigOrDie(config)
	vmInformerFactory := externalversions.NewSharedInformerFactory(vmClientset, 120*time.Second)

	kubeClientset := kubernetes.NewForConfigOrDie(config)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClientset, 120*time.Second)

	stopCh := makeStopChan()

	if *vmCtrl {
		go vm.NewVirtualMachineController(
			vmClientset,
			kubeClientset,
			kubeInformerFactory.Core().V1().Pods(),
			kubeInformerFactory.Batch().V1().Jobs(),
			kubeInformerFactory.Core().V1().Services(),
			kubeInformerFactory.Core().V1().PersistentVolumes(),
			kubeInformerFactory.Core().V1().PersistentVolumeClaims(),
			kubeInformerFactory.Core().V1().Nodes(),
			vmInformerFactory.Virtualmachine().V1alpha1().VirtualMachines(),
			vmInformerFactory.Virtualmachine().V1alpha1().Credentials(),
			vmInformerFactory.Virtualmachine().V1alpha1().Settings(),
			vmInformerFactory.Virtualmachine().V1alpha1().MachineImages(),
			*bridgeIface,
			*noResourceLimits,
		).Run()
	}

	if *ipCtrl {
		go ip.NewIPDiscoveryController(
			vmClientset,
			vmInformerFactory.Virtualmachine().V1alpha1().ARPTables(),
			vmInformerFactory.Virtualmachine().V1alpha1().VirtualMachines(),
			*deviceName,
		).Run(*workers, stopCh)
	}

	if *serv {
		go server.NewServer(
			vmClientset,
			kubeClientset,
			vmInformerFactory.Virtualmachine().V1alpha1().VirtualMachines(),
			kubeInformerFactory.Core().V1().Nodes(),
			vmInformerFactory.Virtualmachine().V1alpha1().Credentials(),
			vmInformerFactory.Virtualmachine().V1alpha1().MachineImages(),
			vmInformerFactory.Virtualmachine().V1alpha1().Settings(),
			*listenAddress,
		).Run(stopCh)
	}

	vmInformerFactory.Start(stopCh)
	kubeInformerFactory.Start(stopCh)

	<-stopCh
}

func NewKubeClientConfig(configPath string) (*rest.Config, error) {
	if configPath != "" {
		return clientcmd.BuildConfigFromFlags("", configPath)
	}
	return rest.InClusterConfig()
}

func makeStopChan() <-chan struct{} {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		glog.Info("Received stop signal, attempting graceful termination...")
		close(stop)
		<-c
		glog.Info("Received stop signal, terminating immediately!")
		os.Exit(1)
	}()
	return stop
}
