package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

type InstanceList struct {
	Instances []*vmapi.VirtualMachine `json:"data"`
}

type Instance struct {
	Name        string             `json:"name"`
	Cpus        int                `json:"cpus"`
	Memory      int                `json:"memory"`
	Image       string             `json:"image"`
	Action      string             `json:"action"`
	PublicKeys  []string           `json:"pubkey"`
	HostedNovnc bool               `json:"novnc"`
	NodeName    string             `json:"node_name"`
	Volume      vmapi.VolumeSource `json:"volume"`
}

func (i *Instance) IsValid() bool {
	return isValidName(i.Name) &&
		isValidCpus(i.Cpus) &&
		isValidMemory(i.Memory) &&
		isValidImage(i.Image) &&
		isValidAction(vmapi.ActionType(i.Action)) &&
		isValidPublicKeys(i.PublicKeys) &&
		isValidNodeName(i.NodeName) &&
		isValidVolume(i.Volume)
}

func (s *server) InstanceGet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	instance, err := s.vmLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(instance)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func (l InstanceList) Len() int           { return len(l.Instances) }
func (l InstanceList) Less(i, j int) bool { return l.Instances[i].Name < l.Instances[j].Name }
func (l InstanceList) Swap(i, j int)      { l.Instances[i], l.Instances[j] = l.Instances[j], l.Instances[i] }

func (s *server) InstanceList(w http.ResponseWriter, r *http.Request) {
	list, err := s.instanceList()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(list)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func (s *server) instanceList() (interface{}, error) {
	vms, err := s.vmLister.List(labels.Everything())
	if err != nil {
		return []byte{}, err
	}

	list := InstanceList{}
	if len(vms) > 0 {
		list.Instances = vms
	} else {
		list.Instances = []*vmapi.VirtualMachine{}
	}
	sort.Sort(list)

	return list, nil
}

type InstanceCreate struct {
	Instance  `json:",inline"`
	Instances int32 `json:"instances"`
}

func (ic *InstanceCreate) IsValid() bool {
	return ic.Instance.IsValid() &&
		isValidInstanceCount(ic.Instances)
}

func (s *server) parseInstanceCreate(w http.ResponseWriter, r *http.Request) *InstanceCreate {
	var ic InstanceCreate
	switch {
	case strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"):
		w.WriteHeader(http.StatusNotImplemented)
		return nil

	default:
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil
		}
		err = json.Unmarshal(body, &ic)
		if err != nil {
			glog.V(3).Infof("error unmarshaling json: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
	}

	if !ic.IsValid() {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	return &ic
}

func (s *server) InstanceCreate(w http.ResponseWriter, r *http.Request) {
	ic := s.parseInstanceCreate(w, r)
	if ic == nil {
		return
	}
	glog.V(5).Infof("Create Instance: %+v", ic)

	if ic.Instances == 1 {
		s.instanceCreateOne(w, r, ic)
	} else {
		s.instanceCreateMany(w, r, ic)
	}
}

func (s *server) createMachineSpec(ic *InstanceCreate) vmapi.VirtualMachineSpec {
	return vmapi.VirtualMachineSpec{
		Cpus:         int32(ic.Cpus),
		MemoryMB:     int32(ic.Memory),
		MachineImage: ic.Image,
		Action:       vmapi.ActionType(ic.Action),
		PublicKeys:   ic.PublicKeys,
		HostedNovnc:  ic.HostedNovnc,
		NodeName:     ic.NodeName,
		Volume:       ic.Instance.Volume,
	}
}

func (s *server) instanceCreateOne(w http.ResponseWriter, r *http.Request, ic *InstanceCreate) {
	vm := &vmapi.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: ic.Name,
		},
		Spec: s.createMachineSpec(ic),
	}

	vm, err := s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Create(vm)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusCreated)
	case apierrors.IsAlreadyExists(err):
		w.WriteHeader(http.StatusConflict)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *server) instanceCreateMany(w http.ResponseWriter, r *http.Request, ic *InstanceCreate) {
	for i := int32(1); i <= ic.Instances; i++ {
		vm := &vmapi.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%02d", ic.Name, i),
			},
			Spec: s.createMachineSpec(ic),
		}

		vm, err := s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Create(vm)
		switch {
		case err == nil:
			continue
		case apierrors.IsAlreadyExists(err):
			w.WriteHeader(http.StatusConflict)
			return
		default:
			glog.V(3).Infof("error creating instance: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *server) parseInstance(w http.ResponseWriter, r *http.Request) *Instance {
	var i Instance
	switch {
	case strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"):
		w.WriteHeader(http.StatusNotImplemented)
		return nil

	default:
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil
		}
		err = json.Unmarshal(body, &i)
		if err != nil {
			glog.V(3).Infof("error unmarshaling json: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
	}

	if !i.IsValid() {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	return &i
}

func (s *server) overlayVMSpec(vm *vmapi.VirtualMachine, i *Instance) *vmapi.VirtualMachine {
	vm2 := vm.DeepCopy()
	vm2.Spec.Cpus = int32(i.Cpus)
	vm2.Spec.MemoryMB = int32(i.Memory)
	vm2.Spec.Action = vmapi.ActionType(i.Action)
	vm2.Spec.PublicKeys = i.PublicKeys
	vm2.Spec.HostedNovnc = i.HostedNovnc
	vm2.Spec.NodeName = i.NodeName
	return vm2
}

func (s *server) updateVMSpec(current *vmapi.VirtualMachine, updated *vmapi.VirtualMachine) (changed bool, err error) {
	if !reflect.DeepEqual(current.Spec, updated.Spec) {
		changed = true
		updated, err = s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Update(updated)
	}
	return
}

func (s *server) InstanceUpdate(w http.ResponseWriter, r *http.Request) {
	i := s.parseInstance(w, r)
	if i == nil {
		return
	}
	glog.V(5).Infof("Update Instance: %+v", i)

	vm, err := s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Get(i.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		vm2 := s.overlayVMSpec(vm, i)
		changed, err := s.updateVMSpec(vm, vm2)

		switch {
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		case changed:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotModified)
		}
	case apierrors.IsNotFound(err):
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

func (s *server) InstanceDelete(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	if !isValidName(name) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Delete(name, &metav1.DeleteOptions{})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case apierrors.IsNotFound(err):
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

type InstanceNames struct {
	Names []string `json:"names"`
}

func parseInstanceNames(w http.ResponseWriter, r *http.Request) *InstanceNames {
	var in InstanceNames
	switch {
	case strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"):
		r.ParseForm()
		if len(r.PostForm["names"]) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		in = InstanceNames{
			Names: r.PostForm["names"],
		}

	default:
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil
		}
		err = json.Unmarshal(body, &in)
		if err != nil {
			glog.V(3).Infof("error unmarshaling json: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
	}
	return &in
}

func (s *server) InstanceDeleteMulti(w http.ResponseWriter, r *http.Request) {
	in := parseInstanceNames(w, r)
	if in == nil {
		return
	}

	if !isValidName(in.Names...) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, name := range in.Names {
		err := s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Delete(name, &metav1.DeleteOptions{})
		switch {
		case err == nil:
			continue
		case apierrors.IsNotFound(err):
			w.WriteHeader(http.StatusNotFound)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) InstanceAction(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	action := mux.Vars(r)["action"]
	actionType := vmapi.ActionType(action)

	if !isValidName(name) || !isValidAction(actionType) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vm, err := s.vmLister.Get(name)
	switch {
	case err == nil:
		break
	case apierrors.IsNotFound(err):
		w.WriteHeader(http.StatusNotFound)
		return
	default:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	vm2 := vm.DeepCopy()
	vm2.Spec.Action = vmapi.ActionType(action)
	if vm.Spec.Action == vm2.Spec.Action {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	vm2, err = s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Update(vm2)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) InstanceActionMulti(w http.ResponseWriter, r *http.Request) {
	action := mux.Vars(r)["action"]
	actionType := vmapi.ActionType(action)
	in := parseInstanceNames(w, r)
	if in == nil {
		return
	}

	if !isValidAction(actionType) || !isValidName(in.Names...) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, name := range in.Names {
		vm, err := s.vmLister.Get(name)
		switch {
		case err == nil:
			break
		case apierrors.IsNotFound(err):
			w.WriteHeader(http.StatusNotFound)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		vm2 := vm.DeepCopy()
		vm2.Spec.Action = vmapi.ActionType(action)
		if vm.Spec.Action == vm2.Spec.Action {
			// In multi scenario we behave idempotently
			continue
		}

		if vm2, err = s.vmClient.VirtualmachineV1alpha1().VirtualMachines().Update(vm2); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
