package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Cpus     int32 `json:"cpus"`
	MemoryMB int32 `json:"memory_mb"`
	// +optional
	MachineImage MachineImageType `json:"image"`
	Action       ActionType       `json:"action"`
	PublicKeys   []string         `json:"public_keys"`
	HostedNovnc  bool             `json:"hosted_novnc"`
	// NodeName is the name of the node where the virtual machine should run.
	// This is mutable at runtime and will trigger a live migration.
	// +optional
	NodeName         string       `json:"node_name"`
	KvmArgs          string       `json:"kvm_extra_args"`
	ImageVMTools     string       `json:"image_vmtools"`
	UseHugePages     bool         `json:"use_hugepages"`
	VmImagePvcName   string       `json:"image_pvc"`
	VmVolumesPvcName string       `json:"volumes_pvc"`
	Volume           VolumeSource `json:"volume"`
}

type VolumeSource struct {
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	Longhorn *LonghornVolumeSource `json:"longhorn,omitempty"`
}

type EmptyDirVolumeSource struct{}

type LonghornVolumeSource struct {
	Size                string `json:"size"`
	BaseImage           string `json:"base_image"`
	NumberOfReplicas    int    `json:"number_of_replicas"`
	StaleReplicaTimeout int    `json:"stale_replica_timeout"`
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
	// StateMigrating indicates the VM is migrating to a new node
	StateMigrating StateType = "migrating"
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
	// NodeName is the name of the node where the virtual machine is running
	NodeName string `json:"node_name"`
	// NodeIP is the IP address of the node where the virtual machine is running
	NodeIP string `json:"node_ip"`
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

type ARPTableStatus struct{}

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

type CredentialStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Credential `json:"items"`
}

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Setting is a generic RancherVM setting
type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SettingSpec   `json:"spec"`
	Status SettingStatus `json:"status"`
}

type SettingSpec struct {
	Value string `json:"value"`
}

type SettingStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Setting `json:"items"`
}

type SettingType string

const (
	SettingTypeString = SettingType("string")
	SettingTypeInt    = SettingType("int")
	SettingTypeBool   = SettingType("bool")
)

type SettingName string

const (
	SettingNameLonghornEndpoint           = SettingName("longhorn-endpoint")
	SettingNameLonghornInsecureSkipVerify = SettingName("longhorn-insecure-skip-verify")
	SettingNameLonghornAccessKey          = SettingName("longhorn-access-key")
	SettingNameLonghornSecretKey          = SettingName("longhorn-secret-key")
)

var (
	SettingNameList = []SettingName{
		SettingNameLonghornEndpoint,
		SettingNameLonghornInsecureSkipVerify,
		SettingNameLonghornAccessKey,
		SettingNameLonghornSecretKey,
	}
)

type SettingCategory string

const (
	SettingCategoryStorage = SettingCategory("storage")
)

type SettingDefinition struct {
	DisplayName string          `json:"displayName"`
	Description string          `json:"description"`
	Category    SettingCategory `json:"category"`
	Type        SettingType     `json:"type"`
	Required    bool            `json:"required"`
	ReadOnly    bool            `json:"readOnly"`
	Default     string          `json:"default"`
}

var (
	SettingDefinitions = map[SettingName]SettingDefinition{
		SettingNameLonghornEndpoint:           SettingDefinitionLonghornEndpoint,
		SettingNameLonghornInsecureSkipVerify: SettingDefinitionLonghornInsecureSkipVerify,
		SettingNameLonghornAccessKey:          SettingDefinitionLonghornAccessKey,
		SettingNameLonghornSecretKey:          SettingDefinitionLonghornSecretKey,
	}

	SettingDefinitionLonghornEndpoint = SettingDefinition{
		DisplayName: "Longhorn Endpoint",
		Description: "The endpoint to Longhorn installation.",
		Category:    SettingCategoryStorage,
		Type:        SettingTypeString,
		Required:    false,
		ReadOnly:    false,
	}

	SettingDefinitionLonghornInsecureSkipVerify = SettingDefinition{
		DisplayName: "Longhorn Insecure Skip Verify",
		Description: "Disable certificate path validation for Longhorn endpoint.",
		Category:    SettingCategoryStorage,
		Type:        SettingTypeBool,
		Required:    false,
		ReadOnly:    false,
		Default:     "false",
	}

	SettingDefinitionLonghornAccessKey = SettingDefinition{
		DisplayName: "Longhorn Access Key",
		Description: "The Rancher API access key for accessing Longhorn installation.",
		Category:    SettingCategoryStorage,
		Type:        SettingTypeString,
		Required:    false,
		ReadOnly:    false,
	}

	SettingDefinitionLonghornSecretKey = SettingDefinition{
		DisplayName: "Longhorn Secret Key",
		Description: "The Rancher API secret key for accessing Longhorn installation.",
		Category:    SettingCategoryStorage,
		Type:        SettingTypeString,
		Required:    false,
		ReadOnly:    false,
	}
)
