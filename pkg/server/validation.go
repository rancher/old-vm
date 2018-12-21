package server

import (
	"regexp"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

var (
	nameRegexp = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
)

func isValidAction(action vmapi.ActionType) bool {
	return action == vmapi.ActionStart ||
		action == vmapi.ActionStop ||
		action == vmapi.ActionReboot
}

func isValidNamespace(namespace string) bool {
	return nameRegexp.MatchString(namespace)
}

func isValidName(names ...string) bool {
	valid := true
	for _, name := range names {
		valid = valid && nameRegexp.MatchString(name)
	}
	return valid
}

func isValidCpus(cpus int) bool {
	return cpus >= 1 && cpus <= 32
}

func isValidMemory(memory int) bool {
	return memory >= 64 && memory <= 65536
}

// TODO
func isValidImage(image string) bool {
	return true
}

func isValidPublicKeys(publicKeys []string) bool {
	valid := true
	for _, publicKey := range publicKeys {
		valid = valid && isValidPublicKey(publicKey)
	}
	return valid
}

// TODO improve
func isValidPublicKey(publicKey string) bool {
	return publicKey != ""
}

func isValidInstanceCount(instanceCount int32) bool {
	return instanceCount >= 1 && instanceCount <= 1024
}

// TODO improve
func isValidNodeName(nodeName string) bool {
	return true
}

func isValidVolume(volume vmapi.VolumeSource) bool {
	if volume.Longhorn != nil && volume.EmptyDir == nil {
		return volume.Longhorn.NumberOfReplicas >= 2 &&
			volume.Longhorn.NumberOfReplicas <= 10 &&
			volume.Longhorn.StaleReplicaTimeout > 0
	} else if volume.EmptyDir != nil && volume.Longhorn == nil {
		return true
	} else if volume.EmptyDir == nil && volume.Longhorn == nil {
		// we assume emptyDir as default volume source for backwards compatibility
		return true
	}
	return false
}
