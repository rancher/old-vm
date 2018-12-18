package vm

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/glog"
	"github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

func GetAlivePods(pods []*corev1.Pod) []*corev1.Pod {
	var alivePods []*corev1.Pod
	for _, pod := range pods {
		if pod.DeletionTimestamp == nil {
			alivePods = append(alivePods, pod)
		}
	}
	return alivePods
}

func IsPodUnschedulable(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			return condition.Reason == corev1.PodReasonUnschedulable
		}
	}
	return false
}

func CreateConsoleProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/sh",
					"-c",
					"[ -S /vm/${MY_POD_NAME}_vnc.sock ]",
				},
			},
		},
		InitialDelaySeconds: 2,
		TimeoutSeconds:      2,
		PeriodSeconds:       3,
		SuccessThreshold:    1,
		FailureThreshold:    10,
	}
}

func (ctrl *VirtualMachineController) createLonghornMachinePod(machine *v1alpha1.VirtualMachine, publicKeys []*v1alpha1.Credential, migrate bool) *corev1.Pod {
	cpu := strconv.Itoa(int(machine.Spec.Cpus))
	mem := strconv.Itoa(int(machine.Spec.MemoryMB))
	kvmextraargs := string(machine.Spec.KvmArgs)

	var imageVmTools string
	if machine.Spec.ImageVMTools == "" {
		imageVmTools = *common.ImageVMTools
	} else {
		imageVmTools = machine.Spec.ImageVMTools
	}
	glog.Infof("imageVmTools: %v", imageVmTools)

	consoleProbe := CreateConsoleProbe()
	podName := newPodName(machine.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// TODO: use GenerateName, find alternative to selector on unique_name
			Name: podName,
			Labels: map[string]string{
				"app":         common.LabelApp,
				"role":        common.LabelRoleVM,
				"name":        machine.Name,
				"unique_name": podName,
			},
			Annotations: map[string]string{
				"cpus":      cpu,
				"memory_mb": mem,
				"id":        machine.Status.ID,
				"mac":       machine.Status.MAC,
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, machine.Name)),
				common.MakeVolHostPath("dev-kvm", "/dev/kvm"),
				common.MakePvcVol("longhorn", machine.Name),
			},
			Containers: []corev1.Container{
				corev1.Container{
					Name:            common.LabelRoleVM,
					Image:           imageVmTools,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"/opt/rancher/vm-tools/startvm"},
					Env: []corev1.EnvVar{
						common.MakeEnvVarFieldPath("MY_POD_NAME", "metadata.name"),
						common.MakeEnvVarFieldPath("MY_POD_NAMESPACE", "metadata.namespace"),
						common.MakeEnvVar("IFACE", ctrl.bridgeIface, nil),
						common.MakeEnvVar("KVM_EXTRA_ARGS", kvmextraargs, nil),
						common.MakeEnvVar("MEMORY_MB", mem, nil),
						common.MakeEnvVar("CPUS", cpu, nil),
						common.MakeEnvVar("MAC", machine.Status.MAC, nil),
						common.MakeEnvVar("INSTANCE_ID", machine.Status.ID, nil),
						common.MakeEnvVar("MIGRATE", strconv.FormatBool(migrate), nil),
						common.MakeEnvVar("MY_VM_NAME", machine.Name, nil),
					},
					VolumeMounts: []corev1.VolumeMount{
						common.MakeVolumeMount("dev-kvm", "/dev/kvm", "", false),
						common.MakeVolumeMount("vm-socket", "/vm", "", false),
						common.MakeVolumeMount("longhorn", "/longhorn", "", false),
					},
					LivenessProbe:  consoleProbe,
					ReadinessProbe: consoleProbe,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
				},
			},
			HostNetwork:      true,
			ImagePullSecrets: ctrl.getImagePullSecrets(),
		},
	}

	// Disallow scheduling a migration pod on the same node
	ctrl.addMachineAntiAffinity(pod, machine)

	ctrl.addResourceRequirements(pod, machine)

	ctrl.addPublicKeys(pod, publicKeys)

	if migrate {
		addMigratePort(pod)
	}

	addNodeAffinity(pod, machine)

	return pod
}

func (ctrl *VirtualMachineController) getImagePullSecrets() (refs []corev1.LocalObjectReference) {
	registrySecret, err := ctrl.settingLister.Get(string(v1alpha1.SettingNameRegistrySecret))
	if err == nil {
		refs = append(refs, corev1.LocalObjectReference{
			Name: registrySecret.Spec.Value,
		})
	} else {
		glog.Warningf("Couldn't get registry secrets: %v", err)
	}
	return
}

var privileged = true

func (ctrl *VirtualMachineController) createMachinePod(machine *v1alpha1.VirtualMachine, publicKeys []*v1alpha1.Credential, image *v1alpha1.MachineImage, migrate bool) *corev1.Pod {
	var hugePagesVolume corev1.Volume
	var machineImageVolume corev1.Volume
	var machineVolumesVolume corev1.Volume

	cpu := strconv.Itoa(int(machine.Spec.Cpus))
	mem := strconv.Itoa(int(machine.Spec.MemoryMB))
	kvmextraargs := string(machine.Spec.KvmArgs)

	consoleProbe := CreateConsoleProbe()

	machineContainer := corev1.Container{
		Name:            common.LabelRoleVM,
		Image:           image.Spec.DockerImage,
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/usr/bin/startvm"},
		Env: []corev1.EnvVar{
			common.MakeEnvVarFieldPath("MY_POD_NAME", "metadata.name"),
			common.MakeEnvVarFieldPath("MY_POD_NAMESPACE", "metadata.namespace"),
			common.MakeEnvVar("IFACE", ctrl.bridgeIface, nil),
			common.MakeEnvVar("KVM_EXTRA_ARGS", kvmextraargs, nil),
			common.MakeEnvVar("MEMORY_MB", mem, nil),
			common.MakeEnvVar("CPUS", cpu, nil),
			common.MakeEnvVar("MAC", machine.Status.MAC, nil),
			common.MakeEnvVar("INSTANCE_ID", machine.Status.ID, nil),
			common.MakeEnvVar("MIGRATE", strconv.FormatBool(migrate), nil),
			common.MakeEnvVar("MY_VM_NAME", machine.Name, nil),
		},
		VolumeMounts: []corev1.VolumeMount{
			common.MakeVolumeMount("vm-image", "/image", "", false),
			common.MakeVolumeMount("vm-volumes", "/volumes", "", false),
			common.MakeVolumeMount("dev-kvm", "/dev/kvm", "", false),
			common.MakeVolumeMount("hugepages", "/hugepages", "", false),
			common.MakeVolumeMount("vm-socket", "/vm", "", false),
			common.MakeVolumeMount("vm-fs", "/bin", "bin", true),
			// kubernetes mounts /etc/hosts, /etc/hostname, /etc/resolv.conf
			// we must grant write permissions to /etc to allow these mounts
			common.MakeVolumeMount("vm-fs", "/etc", "etc", false),
			common.MakeVolumeMount("vm-fs", "/lib", "lib", true),
			common.MakeVolumeMount("vm-fs", "/lib64", "lib64", true),
			common.MakeVolumeMount("vm-fs", "/sbin", "sbin", true),
			common.MakeVolumeMount("vm-fs", "/usr", "usr", true),
			common.MakeVolumeMount("vm-fs", "/var", "var", true),
		},
		LivenessProbe:  consoleProbe,
		ReadinessProbe: consoleProbe,
		// ImagePullPolicy: corev1.PullPolicy{},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	}

	if machine.Spec.UseHugePages {
		hugePagesVolume = common.MakeVolEmptyDirHugePages("hugepages")
	} else {
		hugePagesVolume = common.MakeVolEmptyDir("hugepages")
	}

	if machine.Spec.VmImagePvcName == "" {
		machineImageVolume = common.MakeHostStateVol(machine.Name, "vm-image")
	} else {
		machineImageVolume = common.MakePvcVol("vm-image", machine.Spec.VmImagePvcName)
	}

	if machine.Spec.VmVolumesPvcName == "" {
		machineVolumesVolume = common.MakeHostStateVol(machine.Name, "vm-volumes")
	} else {
		machineVolumesVolume = common.MakePvcVol("vm-volumes", machine.Spec.VmVolumesPvcName)
	}

	var imageVmTools string
	if machine.Spec.ImageVMTools == "" {
		imageVmTools = *common.ImageVMTools
	} else {
		imageVmTools = machine.Spec.ImageVMTools
	}

	uniquePodName := newPodName(machine.Name)
	machinePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			// TODO: use GenerateName, find alternative to selector on unique_name
			Name: uniquePodName,
			Labels: map[string]string{
				"app":         common.LabelApp,
				"name":        machine.Name,
				"unique_name": uniquePodName,
				"role":        common.LabelRoleVM,
			},
			Annotations: map[string]string{
				"cpus":      cpu,
				"memory_mb": mem,
				"id":        machine.Status.ID,
				"mac":       machine.Status.MAC,
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				common.MakeHostStateVol(machine.Name, "vm-fs"),
				machineImageVolume,
				machineVolumesVolume,
				common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, machine.Name)),
				common.MakeVolHostPath("dev-kvm", "/dev/kvm"),
				hugePagesVolume,
			},
			InitContainers: []corev1.Container{
				corev1.Container{
					Name:            "debootstrap",
					Image:           imageVmTools,
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: []corev1.VolumeMount{
						common.MakeVolumeMount("vm-fs", "/vm-tools", "", false),
					},
				},
			},
			Containers: []corev1.Container{
				machineContainer,
			},
			HostNetwork:      true,
			ImagePullSecrets: ctrl.getImagePullSecrets(),
		},
	}

	// Disallow scheduling a migration pod on the same node
	ctrl.addMachineAntiAffinity(machinePod, machine)

	ctrl.addResourceRequirements(machinePod, machine)

	ctrl.addPublicKeys(machinePod, publicKeys)

	if migrate {
		addMigratePort(machinePod)
	}

	addNodeAffinity(machinePod, machine)

	return machinePod
}

func (ctrl *VirtualMachineController) addMachineAntiAffinity(pod *corev1.Pod, machine *v1alpha1.VirtualMachine) {
	pod.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  common.LabelApp,
							"role": common.LabelRoleVM,
							"name": machine.Name,
						},
					},
					TopologyKey: common.LabelNodeHostname,
				},
			},
		},
	}
}

func (ctrl *VirtualMachineController) addResourceRequirements(pod *corev1.Pod, machine *v1alpha1.VirtualMachine) {
	if ctrl.noResourceLimits {
		return
	}
	pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			// CPU, in cores. (500m = .5 cores)
			corev1.ResourceCPU: *resource.NewQuantity(int64(machine.Spec.Cpus), resource.BinarySI),
			// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
			corev1.ResourceMemory: *resource.NewQuantity(int64(machine.Spec.MemoryMB)*1024*1024, resource.BinarySI),
			// Volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024)
			// corev1.ResourceStorage: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
		},
	}
	if machine.Spec.UseHugePages {
		// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
		pod.Spec.Containers[0].Resources.Limits[corev1.ResourceHugePagesPrefix+"2Mi"] =
			*resource.NewQuantity(int64(machine.Spec.MemoryMB)*1024*1024, resource.BinarySI)
	}
}

func (ctrl *VirtualMachineController) getPublicKeys(machine *v1alpha1.VirtualMachine) ([]*v1alpha1.Credential, error) {
	var publicKeys []*v1alpha1.Credential
	for _, publicKeyName := range machine.Spec.PublicKeys {
		publicKey, err := ctrl.credLister.Get(publicKeyName)
		if err != nil {
			return nil, err
		}
		publicKeys = append(publicKeys, publicKey)
	}
	return publicKeys, nil
}

func (ctrl *VirtualMachineController) addPublicKeys(pod *corev1.Pod, publicKeys []*v1alpha1.Credential) {
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
		common.MakeEnvVar("PUBLIC_KEY_COUNT", strconv.Itoa(len(publicKeys)), nil))
	for i, publicKey := range publicKeys {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
			common.MakeEnvVar(fmt.Sprintf("PUBLIC_KEY_%d", i+1), publicKey.Spec.PublicKey, nil))
	}
}

func addMigratePort(pod *corev1.Pod) {
	// TODO this could lead to port conflict in rare circumstance, find a
	// better way. Possibly after MAC VLAN support we can run pod outside
	// host network and use a service to target static migration port.
	migratePort := strconv.Itoa(32768 + (rand.Int() % 32768))

	migratePortVar := common.MakeEnvVar("MIGRATE_PORT", migratePort, nil)
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, migratePortVar)
	pod.ObjectMeta.Annotations["migrate_port"] = migratePort
}

// addNodeAffinity adds a hard affinity constraint to schedule a machine pod onto a
// specific node, if specified. Providing a node name that doesn't exist is
// allowed; pod scheduling will hang until a node with specified name is added.
func addNodeAffinity(pod *corev1.Pod, machine *v1alpha1.VirtualMachine) {
	if machine.Spec.NodeName == "" {
		return
	}
	pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						corev1.NodeSelectorRequirement{
							Key:      common.LabelNodeHostname,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{machine.Spec.NodeName},
						},
					},
				},
			},
		},
	}
}

func newPodName(name string) string {
	return strings.Join([]string{
		name,
		fmt.Sprintf("%08x", rand.Uint32()),
	}, common.NameDelimiter)
}

func makeNovncService(machine *v1alpha1.VirtualMachine) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name + "-novnc",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "novnc",
					Port: 6080,
				},
			},
			Selector: map[string]string{
				"app":  common.LabelApp,
				"name": machine.Name,
				"role": common.LabelRoleNoVNC,
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
}

var noGracePeriod = int64(0)

func (ctrl *VirtualMachineController) makeNovncPod(machine *v1alpha1.VirtualMachine, podName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name + "-novnc",
			Labels: map[string]string{
				"app":  common.LabelApp,
				"name": machine.Name,
				"role": common.LabelRoleNoVNC,
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, machine.Name)),
				common.MakeVolFieldPath("podinfo", "labels", "metadata.labels"),
			},
			Containers: []corev1.Container{
				corev1.Container{
					Name:            common.LabelRoleNoVNC,
					Image:           *common.ImageNoVNC,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"novnc"},
					Env: []corev1.EnvVar{
						common.MakeEnvVar("VM_POD_NAME", podName, nil),
					},
					VolumeMounts: []corev1.VolumeMount{
						common.MakeVolumeMount("vm-socket", "/vm", "", false),
						common.MakeVolumeMount("podinfo", "/podinfo", "", false),
					},
				},
			},
			TerminationGracePeriodSeconds: &noGracePeriod,
			Affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":         common.LabelApp,
									"name":        machine.Name,
									"unique_name": podName,
									"role":        common.LabelRoleVM,
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
			ImagePullSecrets: ctrl.getImagePullSecrets(),
		},
	}
}
