package server

import (
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type NodeList struct {
	Hosts []*corev1.Node `json:"data"`
}

func (s *server) NodeList(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.nodeLister.List(labels.Everything())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(NodeList{
		Hosts: hosts,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}
