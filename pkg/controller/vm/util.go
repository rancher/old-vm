package vm

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

const (
	nameDelimiter = "-"
)

var privileged = true

func (ctrl *VirtualMachineController) makeVMPod(vm *v1alpha1.VirtualMachine, iface string, noResourceLimits bool, migrate bool) *corev1.Pod {
	var publicKeys []*v1alpha1.Credential
	for _, publicKeyName := range vm.Spec.PublicKeys {
		publicKey, err := ctrl.credLister.Get(publicKeyName)
		if err != nil {
			continue
		}
		publicKeys = append(publicKeys, publicKey)
	}

	cpu := strconv.Itoa(int(vm.Spec.Cpus))
	mem := strconv.Itoa(int(vm.Spec.MemoryMB))
	image := string(vm.Spec.MachineImage)

	vncProbe := &corev1.Probe{
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

	vmContainer := corev1.Container{
		Name:            "vm",
		Image:           fmt.Sprintf("rancher/vm-%s", string(vm.Spec.MachineImage)),
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/usr/bin/startvm"},
		Env: []corev1.EnvVar{
			common.MakeEnvVarFieldPath("MY_POD_NAME", "metadata.name"),
			common.MakeEnvVarFieldPath("MY_POD_NAMESPACE", "metadata.namespace"),
			common.MakeEnvVar("IFACE", iface, nil),
			common.MakeEnvVar("MEMORY_MB", mem, nil),
			common.MakeEnvVar("CPUS", cpu, nil),
			common.MakeEnvVar("MAC", vm.Status.MAC, nil),
			common.MakeEnvVar("INSTANCE_ID", vm.Status.ID, nil),
			common.MakeEnvVar("MIGRATE", strconv.FormatBool(migrate), nil),
			common.MakeEnvVar("MY_VM_NAME", vm.Name, nil),
		},
		VolumeMounts: []corev1.VolumeMount{
			common.MakeVolumeMount("vm-image", "/image", "", false),
			common.MakeVolumeMount("dev-kvm", "/dev/kvm", "", false),
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
		LivenessProbe: vncProbe,
		// TODO readinessProbe could be used for checking SSH/RDP/etc
		ReadinessProbe: vncProbe,
		// ImagePullPolicy: corev1.PullPolicy{},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	}

	if !noResourceLimits {
		vmContainer.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				// CPU, in cores. (500m = .5 cores)
				corev1.ResourceCPU: *resource.NewQuantity(int64(vm.Spec.Cpus), resource.BinarySI),
				// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
				corev1.ResourceMemory: *resource.NewQuantity(int64(vm.Spec.MemoryMB)*1024*1024, resource.BinarySI),
				// Volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024)
				// corev1.ResourceStorage: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
			},
		}
	}

	// add public keys to env vars
	vmContainer.Env = append(vmContainer.Env,
		common.MakeEnvVar("PUBLIC_KEY_COUNT", strconv.Itoa(len(publicKeys)), nil))
	for i, publicKey := range publicKeys {
		vmContainer.Env = append(vmContainer.Env,
			common.MakeEnvVar(fmt.Sprintf("PUBLIC_KEY_%d", i+1), publicKey.Spec.PublicKey, nil))
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: newPodName(vm.Name),
			Labels: map[string]string{
				"app":  "ranchervm",
				"name": vm.Name,
				"role": "vm",
			},
			Annotations: map[string]string{
				"cpus":      cpu,
				"memory_mb": mem,
				"image":     image,
				"id":        vm.Status.ID,
				"mac":       vm.Status.MAC,
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				common.MakeHostStateVol(vm.Name, "vm-fs"),
				common.MakeHostStateVol(vm.Name, "vm-image"),
				common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, vm.Name)),
				common.MakeVolHostPath("dev-kvm", "/dev/kvm"),
			},
			InitContainers: []corev1.Container{
				corev1.Container{
					Name:            "debootstrap",
					Image:           "rancher/vm-tools:0.0.3",
					ImagePullPolicy: corev1.PullAlways,
					VolumeMounts: []corev1.VolumeMount{
						common.MakeVolumeMount("vm-fs", "/vm-tools", "", false),
					},
				},
			},
			Containers: []corev1.Container{
				vmContainer,
			},
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":  "ranchervm",
									"name": vm.Name,
									"role": "vm",
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
			HostNetwork: true,
		},
	}
}

func newPodName(name string) string {
	return strings.Join([]string{
		name,
		fmt.Sprintf("%08x", rand.Uint32()),
	}, nameDelimiter)
}

func makeNovncService(vm *v1alpha1.VirtualMachine) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: vm.Name + "-novnc",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "novnc",
					Port: 6080,
				},
			},
			Selector: map[string]string{
				"app":  "ranchervm",
				"name": vm.Name,
				"role": "novnc",
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
}

func makeNovncPod(vm *v1alpha1.VirtualMachine) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: vm.Name + "-novnc",
			Labels: map[string]string{
				"app":  "ranchervm",
				"name": vm.Name,
				"role": "novnc",
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, vm.Name)),
				common.MakeVolFieldPath("podinfo", "labels", "metadata.labels"),
			},
			Containers: []corev1.Container{
				corev1.Container{
					Name:    "novnc",
					Image:   "rancher/novnc:0.0.1",
					Command: []string{"novnc"},
					Env: []corev1.EnvVar{
						common.MakeEnvVarFieldPath("MY_POD_NAMESPACE", "metadata.namespace"),
					},
					VolumeMounts: []corev1.VolumeMount{
						common.MakeVolumeMount("vm-socket", "/vm", "", false),
						common.MakeVolumeMount("podinfo", "/podinfo", "", false),
					},
				},
			},
		},
	}
}
