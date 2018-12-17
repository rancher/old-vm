package vm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	api "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
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

type CreateSnapshotRequest struct{}
type CreateSnapshotResponse struct {
	Name string `json:"name"`
	Size string `json:"size"`
}

func (c *LonghornClient) CreateSnapshot(name string) (*CreateSnapshotResponse, error) {
	req := &CreateSnapshotRequest{}

	buf, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.post("/v1/volumes/"+name+"?action=snapshotCreate", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var cs CreateSnapshotResponse
		if err := json.Unmarshal(buf, &cs); err != nil {
			return nil, err
		}
		return &cs, nil
	}
	return nil, fmt.Errorf("CreateSnapshot failed: %v", resp.Status)
}

type CreateBackupRequest struct {
	SnapshotName string `json:"name"`
}

func (c *LonghornClient) CreateBackup(volumeName, snapshotName string) error {
	req := &CreateBackupRequest{
		SnapshotName: snapshotName,
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := c.post("/v1/volumes/"+volumeName+"?action=snapshotBackup", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CreateBackup failed: %v", resp.Status)
	}
	return nil
}

type BackupListResponse struct {
	Items []*BackupVolume `json:"data"`
}

type BackupVolume struct {
	Name         string            `json:"name"`
	SnapshotName string            `json:"snapshotName"`
	VolumeName   string            `json:"volumeName"`
	URL          string            `json:"url"`
	Labels       map[string]string `json:"labels"`
}

func (c *LonghornClient) GetBackup(volumeName, snapshotName string) (*BackupVolume, error) {
	list, err := c.GetBackupList(volumeName)
	if err != nil {
		return nil, err
	}

	if list == nil || len(list.Items) == 0 {
		return nil, nil
	}

	return list.Items[0], nil
}

func (c *LonghornClient) GetBackupList(volumeName string) (*BackupListResponse, error) {
	resp, err := c.post("/v1/backupvolumes/"+volumeName+"?action=backupList", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var bl BackupListResponse
		if err := json.Unmarshal(buf, &bl); err != nil {
			return nil, err
		}
		return &bl, nil
	}

	return nil, nil
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

func (c *LonghornClient) CreateVolume(machine *api.VirtualMachine, image *api.MachineImage) error {

	vol := &LonghornVolume{
		Name:                machine.Name,
		BaseImage:           image.Spec.DockerImage,
		Size:                strconv.Itoa(image.Spec.SizeGiB) + "Gi",
		Frontend:            machine.Spec.Volume.Longhorn.Frontend,
		NumberOfReplicas:    machine.Spec.Volume.Longhorn.NumberOfReplicas,
		StaleReplicaTimeout: machine.Spec.Volume.Longhorn.StaleReplicaTimeout,
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
