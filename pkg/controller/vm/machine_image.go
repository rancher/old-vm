package vm

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

const (
	DockerContextDir    = "/workspace"
	LonghornEngineImage = "llparse/longhorn-engine:df56c7e-dirty"
	KanikoImage         = "gcr.io/kaniko-project/executor:debug"
	ContainerName       = "create-image"
)

var Privileged = true

func (ctrl *VirtualMachineController) machineImageWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.machineImageQueue.Get()
		if quit {
			return true
		}
		defer ctrl.machineImageQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("machineImageWorker[%s]", key)

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("machineImageWorker[%s] error parsing key: %v", key, err)
			return false
		}

		if err := ctrl.processMachineImage(name); err != nil {
			ctrl.machineImageQueue.AddRateLimited(key)
			glog.Warningf("machineImageWorker[%s] requeued: %v", key, err)
		}
		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("machineImageWorker: shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) processMachineImage(name string) error {
	machineImage, err := ctrl.machineImageLister.Get(name)
	if err != nil {
		return err
	}

	if machineImage.Status.Ready {
		// TODO reassess machine image readiness when nodes enter cluster
		return nil
	}

	// TODO pull the VM CRD to ensure Longhorn-backed
	if machineImage.Spec.FromVirtualMachine != "" {
		if machineImage.Status.Snapshot == "" {
			return ctrl.createSnapshot(machineImage)
		}

		if machineImage.Status.BackupURL == "" {
			return ctrl.createBackup(machineImage)
		}

		if !machineImage.Status.Published {
			return ctrl.publishImage(machineImage)
		}
	}

	// TODO pull image on some/all nodes

	return ctrl.setMachineImageReady(machineImage)
}

func (ctrl *VirtualMachineController) createSnapshot(machineImage *api.MachineImage) error {
	snapshot, err := ctrl.lhClient.CreateSnapshot(machineImage.Spec.FromVirtualMachine)
	if err != nil {
		return err
	}
	mutable := machineImage.DeepCopy()
	mutable.Status.Snapshot = snapshot.Name
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return err
}

func (ctrl *VirtualMachineController) createBackup(machineImage *api.MachineImage) error {
	volumeName := machineImage.Spec.FromVirtualMachine
	snapshotName := machineImage.Status.Snapshot

	backup, err := ctrl.lhClient.GetBackup(volumeName, snapshotName)
	if err != nil {
		return err
	}
	if backup == nil {
		return ctrl.lhClient.CreateBackup(volumeName, snapshotName)
	}

	u, err := url.Parse(backup.URL)
	if err != nil {
		return err
	}
	if len(u.Query()["backup"]) != 1 {
		return errors.New("Invalid backupURL: Expecting one 'backup' query param")
	}
	if len(u.Query()["volume"]) != 1 {
		return errors.New("Invalid backupURL: Expecting one 'volume' query param")
	}

	mutable := machineImage.DeepCopy()
	mutable.Status.BackupURL = backup.URL
	if baseImage, ok := backup.Labels["ranchervm-base-image"]; ok {
		mutable.Status.BaseImage = baseImage
	}
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return err
}

func (ctrl *VirtualMachineController) publishImage(machineImage *api.MachineImage) error {
	pod, err := ctrl.podLister.Pods(common.NamespaceVM).Get(machineImage.Name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			pod = ctrl.getMachineImagePod(machineImage)
			if pod, err = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "kaniko" {
			if status.State.Terminated != nil {
				if status.State.Terminated.ExitCode != 0 {
					return errors.New("Pod failed to create base image")
				}
				if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(machineImage.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
				mutable := machineImage.DeepCopy()
				mutable.Status.Published = true
				mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
				return err
			}
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) setMachineImageReady(machineImage *api.MachineImage) (err error) {
	mutable := machineImage.DeepCopy()
	mutable.Status.Ready = true
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return
}

func (ctrl *VirtualMachineController) getMachineImagePod(machineImage *api.MachineImage) (pod *v1.Pod) {
	imageName := machineImage.Spec.DockerImage

	filename := "base.qcow2"
	outputFile := filepath.Join(DockerContextDir, filename)

	// FIXME we should search for existing pod in case controller died
	glog.V(3).Infof("Creating pod %s/%s", common.NamespaceVM, machineImage.Name)

	pod = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: machineImage.Name,
			Labels: map[string]string{
				"app":  common.LabelApp,
				"name": machineImage.Name,
				"role": common.LabelRoleMachineImage,
			},
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "docker-context",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []v1.Container{
				{
					Name:  "create-dockerfile",
					Image: LonghornEngineImage,
					Command: []string{"/bin/sh", "-c", fmt.Sprintf(
						"echo 'FROM busybox\\nCOPY %s /base_image/' > %s/Dockerfile",
						filename, DockerContextDir,
					)},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "docker-context",
							MountPath: DockerContextDir,
						},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:  ContainerName,
					Image: LonghornEngineImage,
					Command: []string{"/bin/sh", "-c", fmt.Sprintf(
						"longhorn restore-to --backup-url '%s' --output-file '%s'; touch %s/.ready",
						machineImage.Status.BackupURL, outputFile, DockerContextDir)},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "docker-context",
							MountPath: DockerContextDir,
						},
					},
					SecurityContext: &v1.SecurityContext{
						Privileged: &Privileged,
					},
				},
				{
					Name:            "kaniko",
					Image:           KanikoImage,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{"/busybox/sh", "-c", fmt.Sprintf(
						"while true; do if [ -f %s/.ready ]; then break; else sleep 1; fi; done; "+
							"/kaniko/executor --dockerfile=Dockerfile --destination=%s", DockerContextDir, imageName)},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "docker-context",
							MountPath: DockerContextDir,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	ctrl.addRegistryInsecureFlag(pod)

	ctrl.addRegistrySecret(pod)

	ctrl.addBaseImage(pod, machineImage, outputFile)

	return
}

func (ctrl *VirtualMachineController) addRegistryInsecureFlag(pod *v1.Pod) {
	registryInsecureString, err := ctrl.settingLister.Get(string(api.SettingNameRegistryInsecure))
	if registryInsecureString == nil {
		if err != nil {
			glog.Warning(err)
		}
		return
	}

	if registryInsecureString.Spec.Value == "true" {
		pod.Spec.Containers[1].Command[2] = pod.Spec.Containers[1].Command[2] + " --insecure"
	}
}

func (ctrl *VirtualMachineController) addRegistrySecret(pod *v1.Pod) {
	registrySecret, err := ctrl.settingLister.Get(string(api.SettingNameRegistrySecret))
	if registrySecret == nil {
		if err != nil {
			glog.Warning(err)
		}
		return
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		Name: "docker-config",
		VolumeSource: v1.VolumeSource{
			Projected: &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{
					{
						Secret: &v1.SecretProjection{
							LocalObjectReference: v1.LocalObjectReference{
								Name: registrySecret.Spec.Value,
							},
							Items: []v1.KeyToPath{
								{
									Key:  ".dockerconfigjson",
									Path: ".docker/config.json",
								},
							},
						},
					},
				},
			},
		},
	})

	pod.Spec.Containers[1].VolumeMounts = append(pod.Spec.Containers[1].VolumeMounts, v1.VolumeMount{
		Name:      "docker-config",
		MountPath: "/root",
	})
}

func (ctrl *VirtualMachineController) addBaseImage(pod *v1.Pod, machineImage *api.MachineImage, outputFile string) {
	baseImage := machineImage.Status.BaseImage
	if baseImage == "" {
		return
	}

	// Ensure base image is present before executing other containers
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, v1.Container{
		Name:            "prime-base-image",
		Image:           baseImage,
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh", "-c", fmt.Sprintf("echo primed %s", baseImage)},
	})

	// create a volume to propagate the base image bind mount
	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		Name: "share",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	})

	hostToContainer := v1.MountPropagationHostToContainer
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
		Name:             "share",
		ReadOnly:         true,
		MountPath:        "/share",
		MountPropagation: &hostToContainer,
	})
	pod.Spec.Containers[0].Command[2] =
		"while true; do list=$(ls /share/base_image/* 2>&1); if [ $? -eq 0 ]; " +
			"then break; fi; echo waiting; sleep 1; done; echo Directory found $list; " +
			fmt.Sprintf("longhorn restore-to --backing-file /share/base_image "+
				"--backup-url '%s' --output-file '%s'; touch %s/.ready",
				machineImage.Status.BackupURL, outputFile, DockerContextDir)

	bidirectional := v1.MountPropagationBidirectional
	pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
		Name: "base-image",
		Command: []string{"/bin/sh", "-c", "function cleanup() { while true; do " +
			"umount /share/base_image; if [ $? -eq 0 ]; then echo unmounted && " +
			"kill $tpid && break; fi; echo waiting && sleep 1; done }; " +
			"mkdir -p /share/base_image && mount --bind /base_image/ /share/base_image && " +
			"echo base image mounted at /share/base_image && trap cleanup TERM && " +
			"mkfifo noop && tail -f noop & tpid=$! && trap cleanup TERM && wait $tpid"},
		Image:           baseImage,
		ImagePullPolicy: v1.PullNever,
		SecurityContext: &v1.SecurityContext{
			Privileged: &Privileged,
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:             "share",
				MountPath:        "/share",
				MountPropagation: &bidirectional,
			},
		},
	})
}
