package common

const (
	HostStateBaseDir = "/var/lib/rancher/vm"

	FinalizerDeletion = "deletion.vm.rancher.com"
	NamespaceVM       = "default"

	ImageNoVNC    = "rancher/novnc:0.0.1"
	ImageVMTools  = "rancher/vm-tools:0.0.3"
	ImageVMPrefix = "rancher/vm-%s"

	RancherOUI = "06:fe"
)
