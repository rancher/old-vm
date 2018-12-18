package vm

import (
	"strings"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/rancher/vm/pkg/common"
)

func (ctrl *VirtualMachineController) podWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.podQueue.Get()
		if quit {
			return true
		}
		defer ctrl.podQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("podWorker[%s]", key)

		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(2).Infof("podWorker[%s] error parsing key: %v", key, err)
			return false
		}

		pod, err := ctrl.podLister.Pods(ns).Get(name)
		if err != nil && !apierrors.IsNotFound(err) {
			glog.V(2).Infof("podWorker[%s] error getting pod: %v", key, err)
			ctrl.podQueue.AddRateLimited(key)
			glog.V(4).Infof("podWorker[%s] requeued: %v", key, err)
			return false
		}

		ctrl.processPod(pod, key)
		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("pod worker queue shutting down")
			return
		}
	}
}

func (ctrl *VirtualMachineController) processPod(pod *corev1.Pod, key string) {
	if pod == nil {
		return
	}

	if role, ok := pod.Labels["role"]; ok {
		switch role {
		case common.LabelRoleVM:
			fallthrough
		case common.LabelRoleNoVNC:
			machineName := pod.Name[:strings.LastIndex(pod.Name, common.NameDelimiter)]
			ctrl.machineQueue.Add(machineName)
			glog.V(5).Infof("podWorker[%s] enqueued machine: %s", key, machineName)
		case common.LabelRoleMigrate:
		case common.LabelRoleMachineImage:
			name := pod.Name
			if strings.HasPrefix(name, "publish") {
				name = strings.TrimPrefix(name, "publish-")
			} else {
				name = strings.TrimPrefix(name, "pull-")
			}
			name = strings.TrimSuffix(name, "-"+pod.Spec.NodeName)

			ctrl.machineImageQueue.Add(name)
			glog.V(5).Infof("podWorker[%s] enqueued machineImage: %s", key, name)
		}
	}
}
