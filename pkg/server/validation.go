package server

import (
	"regexp"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

var (
	nameRegexp = regexp.MustCompile("^[a-z0-9\\-]{1,128}$")
	nsRegexp   = regexp.MustCompile("^[a-z0-9\\-]{1,64}$")
)

func isValidAction(action vmapi.ActionType) bool {
	return action == vmapi.ActionStart ||
		action == vmapi.ActionStop ||
		action == vmapi.ActionReboot
}

func isValidNamespace(namespace string) bool {
	return nsRegexp.MatchString(namespace)
}

func isValidName(names ...string) bool {
	valid := true
	for _, name := range names {
		valid = valid && nameRegexp.MatchString(name)
	}
	return valid
}

func isValidCpus(cpus int32) bool {
	return cpus >= 1 && cpus <= 32
}

func isValidMemory(memory int32) bool {
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
	return instanceCount >= 1 && instanceCount <= 99
}

// TODO improve
func isValidNodeName(nodeName string) bool {
	return true
}
