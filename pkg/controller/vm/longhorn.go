package vm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	vmlisters "github.com/rancher/vm/pkg/client/listers/ranchervm/v1alpha1"
)

type LonghornClient struct {
	client               *http.Client
	endpoint             string
	accessKey, secretKey string
}

func NewLonghornClient(endpoint, accessKey, secretKey string, insecureSkipVerify bool) *LonghornClient {
	var client *http.Client
	if insecureSkipVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	} else {
		client = http.DefaultClient
	}

	return &LonghornClient{client, endpoint, accessKey, secretKey}
}

func NewLonghornClientFromSettings(settingLister vmlisters.SettingLister) (*LonghornClient, error) {
	endpointSetting, err := settingLister.Get(string(vmapi.SettingNameLonghornEndpoint))
	if err != nil {
		return nil, err
	}
	endpoint := endpointSetting.Spec.Value

	insecureSkipVerifySetting, err := settingLister.Get(string(vmapi.SettingNameLonghornInsecureSkipVerify))
	if err != nil {
		return nil, err
	}
	insecureSkipVerify := insecureSkipVerifySetting.Spec.Value == "true"

	accessKeySetting, err := settingLister.Get(string(vmapi.SettingNameLonghornAccessKey))
	if err != nil {
		return nil, err
	}
	accessKey := accessKeySetting.Spec.Value

	secretKeySetting, err := settingLister.Get(string(vmapi.SettingNameLonghornSecretKey))
	if err != nil {
		return nil, err
	}
	secretKey := secretKeySetting.Spec.Value

	return NewLonghornClient(endpoint, accessKey, secretKey, insecureSkipVerify), nil
}

func (c *LonghornClient) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}

	if c.accessKey != "" && c.secretKey != "" {
		req.SetBasicAuth(c.accessKey, c.secretKey)
	}
	return c.client.Do(req)
}

func (c *LonghornClient) post(path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.endpoint+path, body)
	if err != nil {
		return nil, err
	}

	if c.accessKey != "" && c.secretKey != "" {
		req.SetBasicAuth(c.accessKey, c.secretKey)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *LonghornClient) put(path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.endpoint+path, body)
	if err != nil {
		return nil, err
	}

	if c.accessKey != "" && c.secretKey != "" {
		req.SetBasicAuth(c.accessKey, c.secretKey)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *LonghornClient) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}

	if c.accessKey != "" && c.secretKey != "" {
		req.SetBasicAuth(c.accessKey, c.secretKey)
	}
	return c.client.Do(req)
}

type LonghornVolume struct {
	Name                string `json:"name"`
	Frontend            string `json:"frontend"`
	Size                string `json:"size"`
	BaseImage           string `json:"baseImage"`
	NumberOfReplicas    int    `json:"numberOfReplicas"`
	StaleReplicaTimeout int    `json:"staleReplicaTimeout"`

	Robustness  string       `json:"robustness"`
	State       string       `json:"state"`
	Controllers []Controller `json:"controllers"`
}

type Controller struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	NodeID   string `json:"hostId"`
}

func (c *LonghornClient) CreateVolume(vm *vmapi.VirtualMachine) error {
	vol := &LonghornVolume{
		Name:                vm.Name,
		Frontend:            "iscsi",
		Size:                vm.Spec.Volume.Longhorn.Size,
		BaseImage:           vm.Spec.Volume.Longhorn.BaseImage,
		NumberOfReplicas:    vm.Spec.Volume.Longhorn.NumberOfReplicas,
		StaleReplicaTimeout: vm.Spec.Volume.Longhorn.StaleReplicaTimeout,
	}

	buf, err := json.Marshal(vol)
	if err != nil {
		return err
	}

	resp, err := c.post("/v1/volumes", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CreateVolume failed: %v", resp.Status)
	}
	return nil
}

func (c *LonghornClient) GetVolume(name string) (*LonghornVolume, error) {
	resp, err := c.get("/v1/volumes/" + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var vol LonghornVolume
		if err := json.Unmarshal(buf, &vol); err != nil {
			return nil, err
		}
		return &vol, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("GetVolume failed: %v", resp.Status)
}

type AttachVolumeRequest struct {
	NodeID string `json:"hostId"`
}

func (c *LonghornClient) AttachVolume(name, nodeID string) error {
	req := AttachVolumeRequest{
		NodeID: nodeID,
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := c.post("/v1/volumes/"+name+"?action=attach", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("AttachVolume failed: %v", resp.Status)
}

func (c *LonghornClient) DeleteVolume(name string) error {
	resp, err := c.delete("/v1/volumes/" + name)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DeleteVolume failed: %v", resp.Status)
	}
	return nil
}
