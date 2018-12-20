package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

type MachineImageList struct {
	Items []*v1alpha1.MachineImage `json:"data"`
}

func (l MachineImageList) Len() int           { return len(l.Items) }
func (l MachineImageList) Less(i, j int) bool { return l.Items[i].Name < l.Items[j].Name }
func (l MachineImageList) Swap(i, j int)      { l.Items[i], l.Items[j] = l.Items[j], l.Items[i] }

func (s *server) MachineImageList(w http.ResponseWriter, r *http.Request) {
	list, err := s.machineImageList()
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

func (s *server) machineImageList() (interface{}, error) {
	machineImages, err := s.machineImageLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	list := MachineImageList{}
	if len(machineImages) > 0 {
		list.Items = machineImages
	} else {
		list.Items = []*v1alpha1.MachineImage{}
	}
	sort.Sort(list)

	return list, nil
}

type MachineImageCreate struct {
	v1alpha1.MachineImageSpec `json:",inline"`
	Name                      string `json:"name"`
}

func (s *server) MachineImageCreate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var mic MachineImageCreate
	err = json.Unmarshal(body, &mic)
	if err != nil {
		glog.V(3).Infof("error unmarshaling json: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	machineImage := &v1alpha1.MachineImage{
		// I shouldn't have to set the type meta, what's wrong with the client?
		TypeMeta: metav1.TypeMeta{
			APIVersion: "vm.rancher.io/v1alpha1",
			Kind:       "MachineImage",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: mic.Name,
		},
		Spec: v1alpha1.MachineImageSpec{
			DockerImage:        mic.DockerImage,
			SizeGiB:            mic.SizeGiB,
			FromVirtualMachine: mic.FromVirtualMachine,
		},
	}

	_, err = s.vmClient.VirtualmachineV1alpha1().MachineImages().Create(machineImage)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusCreated)
	case apierrors.IsAlreadyExists(err):
		w.WriteHeader(http.StatusConflict)
	default:
		logrus.Warningf("Error creating machine image: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *server) MachineImageGet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	machineImage, err := s.machineImageLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(machineImage)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)

}

func (s *server) MachineImageDelete(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	err := s.vmClient.VirtualmachineV1alpha1().MachineImages().Delete(name, &metav1.DeleteOptions{})
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
