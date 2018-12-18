package vm

import (
	"strings"

	"github.com/golang/glog"
	"k8s.io/client-go/tools/cache"

	"github.com/rancher/vm/pkg/common"
)

func (ctrl *VirtualMachineController) jobWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.jobQueue.Get()
		if quit {
			return true
		}
		defer ctrl.jobQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("jobWorker[%s]", key)

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(2).Infof("jobWorker[%s] error parsing key: %v", key, err)
			return false
		}

		machineName := name[:strings.LastIndex(name, common.NameDelimiter)]
		ctrl.machineQueue.Add(machineName)
		glog.V(5).Infof("jobWorker[%s] enqueued machine: %s", key, machineName)

		return false
	}
	for {
		if quit := workFunc(); quit {
			glog.Infof("jobWorker: shutting down")
			return
		}
	}
}
