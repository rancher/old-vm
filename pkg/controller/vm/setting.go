package vm

import (
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
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
			glog.Infof("settingWorker: shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) initializeSettings() error {
	for name, definition := range api.SettingDefinitions {
		setting, err := ctrl.settingLister.Get(string(name))
		if apierrors.IsNotFound(err) || setting == nil {
			setting := &api.Setting{
				// I shouldn't have to set the type meta, what's wrong with the client?
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vm.rancher.io/v1alpha1",
					Kind:       "Setting",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: string(name),
				},
				Spec: api.SettingSpec{
					Value: definition.Default,
				},
			}
			if _, err := ctrl.vmClient.VirtualmachineV1alpha1().Settings().Create(setting); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
