package common

import (
	"flag"
)

const (
	HostStateBaseDir = "/var/lib/rancher/vm"

	FinalizerDeletion = "deletion.vm.rancher.io"
	NamespaceVM       = "default"
	NameDelimiter     = "-"

	RancherOUI = "06:fe"

	LabelApp              = "ranchervm"
	LabelRoleMigrate      = "migrate"
	LabelRoleVM           = "vm"
	LabelRoleNoVNC        = "novnc"
	LabelRoleMachineImage = "machineimage"
	LabelNodeHostname     = "kubernetes.io/hostname"
)

var (
	ImageVM      = flag.String("image-vm", "rancher/vm:latest", "VM Docker Image")
	ImageNoVNC   = flag.String("image-novnc", "rancher/vm-novnc:latest", "NoVNC Docker Image")
	ImageVMTools = flag.String("image-tools", "rancher/vm-tools:latest", "Tools Docker Image")
)
