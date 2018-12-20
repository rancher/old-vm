package vm

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

const DockerContextDir = "/workspace"

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
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if machineImage.Spec.FromVirtualMachine != "" {
		machine, err := ctrl.machineLister.Get(machineImage.Spec.FromVirtualMachine)
		if err != nil {
			return err
		}

		if machineImage.Spec.SizeGiB == 0 {
			parentImage, err := ctrl.machineImageLister.Get(machine.Spec.MachineImage)
			if err != nil {
				return err
			}
			return ctrl.setMachineImageSize(machineImage, parentImage.Spec.SizeGiB)
		}

		if machine.Spec.Volume.Longhorn == nil {
			err := errors.New(fmt.Sprintf("machine (%s) referenced by machine image (%s) missing Longhorn volume (%+v)",
				machine.Name, machineImage.Name, machine.Spec.Volume))
			glog.Error(err)
			return err
		}

		if machineImage.Status.Snapshot == "" {
			if machineImage.Status.State != api.MachineImageSnapshot {
				return ctrl.setMachineImageState(machineImage, api.MachineImageSnapshot)
			}
			return ctrl.createSnapshot(machineImage)
		}

		if machineImage.Status.BackupURL == "" {
			if machineImage.Status.State != api.MachineImageBackup {
				return ctrl.setMachineImageState(machineImage, api.MachineImageBackup)
			}
			return ctrl.createBackup(machineImage)
		}

		if !machineImage.Status.Published {
			if machineImage.Status.State != api.MachineImagePublish {
				return ctrl.setMachineImageState(machineImage, api.MachineImagePublish)
			}
			return ctrl.publishImage(machineImage)
		}
	} else {
		if !machineImage.Status.Published {
			return ctrl.setMachineImagePublished(machineImage)
		}
	}

	return ctrl.provisionNodes(machineImage)
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

	// FIXME sometimes picks wrong backup
	backup, err := ctrl.lhClient.GetBackup(volumeName, snapshotName)
	if err != nil {
		return err
	}
	if backup == nil {
		if err := ctrl.lhClient.CreateBackup(volumeName, snapshotName); err != nil {
			return err
		}
		backup, err = ctrl.lhClient.GetBackup(volumeName, snapshotName)
		if err != nil {
			return err
		}
	}
	if backup == nil {
		ctrl.machineImageQueue.AddRateLimited(machineImage.Name)
		return nil
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
	publishPodName := getPublishImagePodName(machineImage)
	pod, err := ctrl.podLister.Pods(common.NamespaceVM).Get(publishPodName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		pod, err = ctrl.getPublishImagePod(machineImage)
		if err != nil {
			return err
		}
		if pod, err = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod); err != nil {
			return err
		}
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "kaniko" {
			if status.State.Terminated != nil {
				if status.State.Terminated.ExitCode != 0 {
					return errors.New("Pod failed to create base image")
				}
				publishPodName := getPublishImagePodName(machineImage)
				if err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(publishPodName, &metav1.DeleteOptions{}); err != nil {
					return err
				}
				return ctrl.setMachineImagePublished(machineImage)
			}
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) provisionNodes(machineImage *api.MachineImage) error {
	nodes, err := ctrl.nodeLister.List(labels.Everything())
	if err != nil {
		return err
	}

	nodesReady := []string{}
	for _, node := range nodes {
		pullPodName := getPullImagePodName(machineImage, node)
		if nodeContainsImage(node, machineImage) {
			nodesReady = append(nodesReady, node.Name)
			err := ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Delete(pullPodName, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}

		pod, err := ctrl.podLister.Pods(common.NamespaceVM).Get(pullPodName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		} else if pod != nil {
			switch pod.Status.Phase {
			case v1.PodPending:
			case v1.PodRunning:
			case v1.PodSucceeded:
				nodesReady = append(nodesReady, node.Name)
				if err := ctrl.kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
			case v1.PodFailed:
				fallthrough
			case v1.PodUnknown:
				glog.Warningf("Pull image pod error: %+v", pod.Status)
				if err := ctrl.kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
			}
			continue
		}

		if nodeContainsImage(node, machineImage) {
			nodesReady = append(nodesReady, node.Name)
		} else if pod == nil {
			glog.V(4).Infof("Node (%s) missing image (%s)", node.Name, machineImage.Spec.DockerImage)
			pod := getPullImagePod(machineImage, node)
			if pod, err = ctrl.kubeClient.CoreV1().Pods(common.NamespaceVM).Create(pod); err != nil {
				return err
			}
		}
	}
	sort.Strings(nodesReady)

	minimumAvailability, err := ctrl.settingLister.Get(string(api.SettingNameImageMinimumAvailability))
	if err != nil {
		return err
	}
	minNodeReadyCount, err := strconv.Atoi(minimumAvailability.Spec.Value)
	if err != nil {
		return err
	}

	currentState := api.MachineImageProvision
	if len(nodesReady) >= minNodeReadyCount || len(nodesReady) == len(nodes) {
		currentState = api.MachineImageReady
	}
	return ctrl.updateNodesReady(machineImage, currentState, nodesReady)
}

func getPullImagePodName(machineImage *api.MachineImage, node *v1.Node) string {
	return strings.Join([]string{"pull", machineImage.Name, node.Name}, "-")
}

func getPullImagePod(machineImage *api.MachineImage, node *v1.Node) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: getPullImagePodName(machineImage, node),
			Labels: map[string]string{
				"app":  common.LabelApp,
				"name": machineImage.Name,
				"role": common.LabelRoleMachineImage,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "pull",
					Image:           machineImage.Spec.DockerImage,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{"/bin/sh", "-c",
						"echo Pulled successfully"},
				},
			},
			NodeName:      node.Name,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

func nodeContainsImage(node *v1.Node, machineImage *api.MachineImage) bool {
	for _, nodeName := range machineImage.Status.Nodes {
		if nodeName == node.Name {
			return true
		}
	}
	return false
}

func (ctrl *VirtualMachineController) setMachineImagePublished(machineImage *api.MachineImage) (err error) {
	if machineImage.Status.Published {
		return
	}
	mutable := machineImage.DeepCopy()
	mutable.Status.Published = true
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return
}

func (ctrl *VirtualMachineController) setMachineImageSize(machineImage *api.MachineImage, size int) (err error) {
	mutable := machineImage.DeepCopy()
	mutable.Spec.SizeGiB = size
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return
}

func (ctrl *VirtualMachineController) setMachineImageState(machineImage *api.MachineImage, state api.MachineImageState) (err error) {
	mutable := machineImage.DeepCopy()
	mutable.Status.State = state
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	return
}

func (ctrl *VirtualMachineController) updateNodesReady(machineImage *api.MachineImage, state api.MachineImageState, nodesReady []string) (err error) {
	oldState := machineImage.Status.State
	mutable := machineImage.DeepCopy()
	mutable.Status.State = state
	mutable.Status.Nodes = nodesReady
	sort.Strings(mutable.Status.Nodes)
	mutable, err = ctrl.vmClient.VirtualmachineV1alpha1().MachineImages().Update(mutable)
	if err == nil && oldState == api.MachineImageProvision && state == api.MachineImageReady {
		machines, err := ctrl.machineLister.List(labels.Everything())
		if err != nil {
			return err
		}
		for _, machine := range machines {
			if machine.Spec.MachineImage == machineImage.Name {
				ctrl.machineQueue.Add(machine.Name)
			}
		}
	}
	return
}

func getPublishImagePodName(machineImage *api.MachineImage) string {
	return "publish-" + machineImage.Name
}

func (ctrl *VirtualMachineController) getPublishImagePod(machineImage *api.MachineImage) (*v1.Pod, error) {
	longhornImage, err := ctrl.settingLister.Get(string(api.SettingNameImageLonghornEngine))
	if err != nil {
		return nil, err
	}

	kanikoImage, err := ctrl.settingLister.Get(string(api.SettingNameImageKaniko))
	if err != nil {
		return nil, err
	}

	filename := "base.qcow2"
	createDockerfile := fmt.Sprintf("echo -e 'FROM busybox\\nCOPY %s /base_image/'"+
		" > %s/Dockerfile", filename, DockerContextDir)
	outputFile := filepath.Join(DockerContextDir, filename)

	glog.V(3).Infof("Creating pod %s/%s", common.NamespaceVM, machineImage.Name)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: getPublishImagePodName(machineImage),
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
					Name:    "create-dockerfile",
					Image:   kanikoImage.Spec.Value,
					Command: []string{"/busybox/sh", "-c", createDockerfile},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "docker-context",
							MountPath: DockerContextDir,
						},
					},
				},
			},
			Containers: []v1.Container{
				ctrl.getLonghornContainer(machineImage, longhornImage, outputFile),
				ctrl.getKanikoContainer(machineImage, kanikoImage),
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	if err := ctrl.addRegistryInsecureFlag(pod); err != nil {
		return nil, err
	}

	if err := ctrl.addRegistrySecret(pod); err != nil {
		return nil, err
	}

	ctrl.addBaseImage(pod, machineImage, outputFile)

	return pod, nil
}

func (ctrl *VirtualMachineController) getLonghornContainer(machineImage *api.MachineImage, longhornImage *api.Setting, outputFile string) v1.Container {
	return v1.Container{
		Name:  "longhorn",
		Image: longhornImage.Spec.Value,
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
	}
}

func (ctrl *VirtualMachineController) getKanikoContainer(machineImage *api.MachineImage, kanikoImage *api.Setting) v1.Container {
	return v1.Container{
		Name:  "kaniko",
		Image: kanikoImage.Spec.Value,
		Command: []string{"/busybox/sh", "-c", fmt.Sprintf(
			"while true; do if [ -f %s/.ready ]; then break; else sleep 1; fi; done; "+
				"/kaniko/executor --dockerfile=Dockerfile --destination=%s",
			DockerContextDir, machineImage.Spec.DockerImage)},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "docker-context",
				MountPath: DockerContextDir,
			},
		},
	}
}

func (ctrl *VirtualMachineController) addRegistryInsecureFlag(pod *v1.Pod) error {
	registryInsecure, err := ctrl.settingLister.Get(string(api.SettingNameRegistryInsecure))
	if err != nil {
		return err
	}

	if registryInsecure.Spec.Value == "true" {
		pod.Spec.Containers[1].Command[2] = pod.Spec.Containers[1].Command[2] + " --insecure"
	}
	return nil
}

func (ctrl *VirtualMachineController) addRegistrySecret(pod *v1.Pod) error {
	registrySecret, err := ctrl.settingLister.Get(string(api.SettingNameRegistrySecret))
	if err != nil {
		return err
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
	return nil
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
