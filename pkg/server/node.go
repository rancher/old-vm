package server

import (
	"encoding/json"
	"net/http"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type NodeList struct {
	Nodes []*corev1.Node `json:"data"`
}

func (l NodeList) Len() int           { return len(l.Nodes) }
func (l NodeList) Less(i, j int) bool { return l.Nodes[i].Name < l.Nodes[j].Name }
func (l NodeList) Swap(i, j int)      { l.Nodes[i], l.Nodes[j] = l.Nodes[j], l.Nodes[i] }

func (s *server) NodeList(w http.ResponseWriter, r *http.Request) {
	list, err := s.nodeList()
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

func (s *server) nodeList() (interface{}, error) {
	nodes, err := s.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	list := NodeList{}
	if len(nodes) > 0 {
		list.Nodes = nodes
	} else {
		list.Nodes = []*corev1.Node{}
	}
	sort.Sort(list)

	return list, nil
}
