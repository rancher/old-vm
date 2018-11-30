package vm

import (
	"github.com/golang/glog"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

func (ctrl *VirtualMachineController) settingWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.settingQueue.Get()
		if quit {
			return true
		}
		defer ctrl.settingQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("settingWorker[%s]", key)

		if err := ctrl.updateLonghornClient(); err != nil {
			ctrl.settingQueue.AddRateLimited(key)
			glog.Warningf("settingWorker[%s] requeued: %v", key, err)
		}
		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("pod worker queue shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) updateLonghornClient() error {
	endpointSetting, err := ctrl.settingLister.Get(string(vmapi.SettingNameLonghornEndpoint))
	if err != nil {
		return err
	}
	endpoint := endpointSetting.Spec.Value

	accessKeySetting, err := ctrl.settingLister.Get(string(vmapi.SettingNameLonghornAccessKey))
	if err != nil {
		return err
	}
	accessKey := accessKeySetting.Spec.Value

	secretKeySetting, err := ctrl.settingLister.Get(string(vmapi.SettingNameLonghornSecretKey))
	if err != nil {
		return err
	}
	secretKey := secretKeySetting.Spec.Value

	insecureSkipVerifySetting, err := ctrl.settingLister.Get(string(vmapi.SettingNameLonghornInsecureSkipVerify))
	if err != nil {
		return err
	}
	insecureSkipVerify := insecureSkipVerifySetting.Spec.Value == "true"

	ctrl.lhClient = NewLonghornClient(endpoint, accessKey, secretKey, insecureSkipVerify)
	return nil
}
