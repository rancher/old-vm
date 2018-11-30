package common

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// IsPodReady returns the PodReady condition status as a boolean
func IsPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			return true
		default:
			return false
		}
	}
	return false
}

func MakeEnvVar(name, value string, valueFrom *corev1.EnvVarSource) corev1.EnvVar {
	return corev1.EnvVar{
		Name:      name,
		Value:     value,
		ValueFrom: valueFrom,
	}
}

func MakeEnvVarFieldPath(name, fieldPath string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: fieldPath,
			},
		},
	}
}

func MakeVolEmptyDir(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func MakeVolEmptyDirHugePages(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumHugePages,
			},
		},
	}
}

func MakeVolHostPath(name, path string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
			},
		},
	}
}

func MakeVolFieldPath(name, path, fieldPath string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					corev1.DownwardAPIVolumeFile{
						Path: path,
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: fieldPath,
						},
					},
				},
			},
		},
	}
}

func MakeVolumeMount(name, mountPath, subPath string, readOnly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:             name,
		ReadOnly:         readOnly,
		MountPath:        mountPath,
		SubPath:          subPath,
		MountPropagation: nil,
	}
}

func MakeHostStateVol(vmName, volName string) corev1.Volume {
	return corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/%s/%s", HostStateBaseDir, vmName, volName),
			},
		},
	}
}

func MakePvcVol(name, pvcname string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcname,
			},
		},
	}
}
