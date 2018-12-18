package qemu

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

func NewMigrationJob(vm *v1alpha1.VirtualMachine, podName, targetURI string, imagePullSecrets []corev1.LocalObjectReference) *batchv1.Job {
	objectMeta := metav1.ObjectMeta{
		Name: fmt.Sprintf("%s-migrate", vm.Name),
		Labels: map[string]string{
			"app":  common.LabelApp,
			"name": vm.Name,
			"role": common.LabelRoleMigrate,
		},
	}

	return &batchv1.Job{
		ObjectMeta: objectMeta,
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objectMeta,
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						common.MakeVolHostPath("vm-socket", fmt.Sprintf("%s/%s", common.HostStateBaseDir, vm.Name)),
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:            common.LabelRoleMigrate,
							Image:           *common.ImageVM,
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"sh", "-c"},
							Args:            []string{fmt.Sprintf("exec /ranchervm -migrate -sock-path /vm/%s_monitor.sock -target-uri %s -v 5", podName, targetURI)},
							VolumeMounts: []corev1.VolumeMount{
								common.MakeVolumeMount("vm-socket", "/vm", "", false),
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app":         common.LabelApp,
											"name":        vm.Name,
											"unique_name": podName,
											"role":        common.LabelRoleVM,
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					ImagePullSecrets: imagePullSecrets,
				},
			},
		},
	}
}
