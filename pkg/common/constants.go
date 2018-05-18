package common

const (
	HostStateBaseDir = "/var/lib/rancher/vm"

	FinalizerDeletion = "deletion.vm.rancher.com"
	NamespaceVM       = "default"
	NameDelimiter     = "-"

	ImageVM       = "rancher/vm"
	ImageVMPrefix = "rancher/vm-%s"
	ImageVMTools  = "rancher/vm-tools"
	ImageNoVNC    = "rancher/novnc"

	RancherOUI = "06:fe"

	LabelApp          = "ranchervm"
	LabelRoleMigrate  = "migrate"
	LabelRoleVM       = "vm"
	LabelRoleNoVNC    = "novnc"
	LabelNodeHostname = "kubernetes.io/hostname"
)
