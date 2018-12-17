package vm

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	"github.com/rancher/vm/pkg/common"
)

func (ctrl *VirtualMachineController) updateLonghornClient() error {
	endpointSetting, err := ctrl.settingLister.Get(string(api.SettingNameLonghornEndpoint))
	if err != nil {
		return err
	}
	endpoint := strings.TrimSuffix(endpointSetting.Spec.Value, "/")

	accessKeySetting, err := ctrl.settingLister.Get(string(api.SettingNameLonghornAccessKey))
	if err != nil {
		return err
	}
	accessKey := accessKeySetting.Spec.Value

	secretKeySetting, err := ctrl.settingLister.Get(string(api.SettingNameLonghornSecretKey))
	if err != nil {
		return err
	}
	secretKey := secretKeySetting.Spec.Value

	insecureSkipVerifySetting, err := ctrl.settingLister.Get(string(api.SettingNameLonghornInsecureSkipVerify))
	if err != nil {
		return err
	}
	insecureSkipVerify := insecureSkipVerifySetting.Spec.Value == "true"

	ctrl.lhClient = NewLonghornClient(endpoint, accessKey, secretKey, insecureSkipVerify)
	return nil
}

func (ctrl *VirtualMachineController) createLonghornVolume(machine *api.VirtualMachine) error {
	image, err := ctrl.machineImageLister.Get(machine.Spec.MachineImage)
	if err != nil {
		return err
	}

	if image.Status.State != api.MachineImageReady {
		return fmt.Errorf("Machine image state: %s", image.Status.State)
	}

	if vol, err := ctrl.lhClient.GetVolume(machine.Name); err != nil {
		return err
	} else if vol == nil {
		if err := ctrl.lhClient.CreateVolume(machine, image); err != nil {
			return err
		}
	}

	if _, err := ctrl.pvLister.Get(machine.Name); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := ctrl.createPersistentVolume(machine, image); err != nil {
			return err
		}
	}

	if _, err := ctrl.pvcLister.PersistentVolumeClaims(common.NamespaceVM).Get(machine.Name); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := ctrl.createPersistentVolumeClaim(machine, image); err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) deleteLonghornVolume(machine *api.VirtualMachine) error {
	if vol, err := ctrl.lhClient.GetVolume(machine.Name); err != nil {
		return err
	} else if vol != nil {
		if err := ctrl.lhClient.DeleteVolume(machine.Name); err != nil {
			return err
		}
	}

	// FIXME maybe don't delete both?
	if err := ctrl.kubeClient.CoreV1().PersistentVolumes().Delete(machine.Name, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(common.NamespaceVM).Delete(machine.Name, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (ctrl *VirtualMachineController) createPersistentVolume(machine *api.VirtualMachine, image *api.MachineImage) error {

	_, err := ctrl.kubeClient.CoreV1().PersistentVolumes().Create(&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name,
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Capacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceStorage: resource.MustParse(strconv.Itoa(image.Spec.SizeGiB) + "Gi"),
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "io.rancher.longhorn",
					VolumeAttributes: map[string]string{
						"frontend": "iscsi",
					},
					VolumeHandle: machine.Name,
				},
			},
		},
	})
	return err
}

var noStorageClass = ""

func (ctrl *VirtualMachineController) createPersistentVolumeClaim(machine *api.VirtualMachine, image *api.MachineImage) error {
	_, err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(common.NamespaceVM).Create(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse(strconv.Itoa(image.Spec.SizeGiB) + "Gi"),
				},
			},
			StorageClassName: &noStorageClass,
			VolumeName:       machine.Name,
		},
	})
	return err
}
