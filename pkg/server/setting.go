package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

type SettingList struct {
	Items []*v1alpha1.Setting `json:"data"`
}

func (l SettingList) Len() int           { return len(l.Items) }
func (l SettingList) Less(i, j int) bool { return l.Items[i].Name < l.Items[j].Name }
func (l SettingList) Swap(i, j int)      { l.Items[i], l.Items[j] = l.Items[j], l.Items[i] }

func (s *server) SettingList(w http.ResponseWriter, r *http.Request) {
	list, err := s.settingList()
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

func (s *server) settingList() (interface{}, error) {
	settings, err := s.settingLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	list := SettingList{}
	if len(settings) > 0 {
		list.Items = settings
	} else {
		list.Items = []*v1alpha1.Setting{}
	}
	sort.Sort(list)

	return list, nil
}

func (s *server) SettingGet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	setting, err := s.settingLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(setting)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)

}

type SettingSet struct {
	Value string `json:"value"`
}

func (s *server) SettingSet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var ss SettingSet
	err = json.Unmarshal(body, &ss)
	if err != nil {
		glog.V(3).Infof("error unmarshaling json: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	setting, err := s.settingLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			setting := &v1alpha1.Setting{
				// I shouldn't have to set the type meta, what's wrong with the client?
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vm.rancher.io/v1alpha1",
					Kind:       "Setting",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: v1alpha1.SettingSpec{
					Value: ss.Value,
				},
			}
			if setting, err = s.vmClient.VirtualmachineV1alpha1().Settings().Create(setting); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	mutable := setting.DeepCopy()
	mutable.Spec.Value = ss.Value
	if mutable, err = s.vmClient.VirtualmachineV1alpha1().Settings().Update(mutable); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
	return
}
