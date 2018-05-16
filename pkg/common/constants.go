package common

const (
	HostStateBaseDir = "/var/lib/rancher/vm"

	FinalizerDeletion = "deletion.vm.rancher.com"
	NamespaceVM       = "default"

	ImageVM       = "rancher/vm"
	ImageVMPrefix = "rancher/vm-%s"
	ImageVMTools  = "rancher/vm-tools:0.0.3"
	ImageNoVNC    = "rancher/novnc:0.0.1"

	RancherOUI = "06:fe"

	LabelApp         = "ranchervm"
	LabelRoleMigrate = "migrate"
	LabelRoleVM      = "vm"
	LabelRoleNoVNC   = "novnc"
)
