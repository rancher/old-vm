package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachine is a specification for a VirtualMachine resource
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec"`
	Status VirtualMachineStatus `json:"status"`
}

type MachineImageType string

const (
	MachineImageAndroidX86 MachineImageType = "android-x86"
	MachineImageCentOS     MachineImageType = "centos"
	MachineImageRancherOS  MachineImageType = "rancheros"
	MachineImageUbuntu     MachineImageType = "ubuntu"
	MachineImageWindows7   MachineImageType = "windows7"
)

type ActionType string

const (
	ActionStart  ActionType = "start"
	ActionStop   ActionType = "stop"
	ActionReboot ActionType = "reboot"
)

// VirtualMachineSpec is the spec for a VirtualMachine resource
type VirtualMachineSpec struct {
	Cpus         int32            `json:"cpus"`
	MemoryMB     int32            `json:"memory_mb"`
	MachineImage MachineImageType `json:"image"`
	Action       ActionType       `json:"action"`
	PublicKeys   []string         `json:"public_keys"`
	HostedNovnc  bool             `json:"hosted_novnc"`
	Disks        VDisk            `json:"disks"`
}

type VDisk struct {
	Root bool
}

type StateType string

const (
	// StatePending indicates a VM is booting
	StatePending StateType = "pending"
	// StateRunning indicates a VM is running. The vnc port and/or ssh port
	// must be accessible for a VM in this state.
	StateRunning StateType = "running"
	// StateStopping indicates a VM is gracefully shutting down
	StateStopping StateType = "stopping"
	// StateStopped indicates an already-created VM is not currently running
	StateStopped StateType = "stopped"
	// StateTerminating indicates the VM is being deleted
	StateTerminating StateType = "terminating"
	// StateTerminated indicates the VM is deleted. The Root block device
	// belonging to the VM may or may not be deleted.
	StateTerminated StateType = "terminated"
	// StateError indicates something went horribly wrong and we are not sure
	// how to proceed
	StateError StateType = "error"
)

// VirtualMachineStatus is the status for a VirtualMachine resource
type VirtualMachineStatus struct {
	// State is the current state of the virtual machine
	State StateType `json:"state"`
	// VncEndpoint is an endpoint exposing a NoVNC webserver
	VncEndpoint string `json:"vnc_endpoint"`
	// ID is an external unique identifier for the virtual machine. It is derived
	// from the metadata uid field.
	ID string `json:"id"`
	// MAC address we will assign to a guest NIC, if necessary. It is derived
	// from the metadata uid field.
	MAC string `json:"mac"`
	// IP address assigned to the guest NIC
	IP string `json:"ip"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachineList is a list of VirtualMachine resources
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VirtualMachine `json:"items"`
}

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ARPTable is a set of ip/mac correlations discovered on a node's host network.
// It is used to deduce what IP address is assigned to a VM without accessing
// the DHCP server or adding instrumentation to each VM.
type ARPTable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ARPTableSpec   `json:"spec"`
	Status ARPTableStatus `json:"status"`
}

type ARPEntry struct {
	IP        string `json:"ip"`
	HWType    string `json:"hw_type"`
	Flags     string `json:"flags"`
	HWAddress string `json:"hw_addr"`
	Mask      string `json:"mask"`
	Device    string `json:"device"`
}

type ARPTableSpec struct {
	Table map[string]ARPEntry `json:"table"`
}

type ARPTableStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ARPTableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ARPTable `json:"items"`
}

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Credential is a public key that may be used to connect to VMs
type Credential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CredentialSpec   `json:"spec"`
	Status CredentialStatus `json:"status"`
}

type CredentialSpec struct {
	PublicKey string `json:"public_key"`
}

type CredentialStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Credential `json:"items"`
}
