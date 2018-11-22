package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rancher/vm/pkg/server"

	vmapi "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
)

func (c *RancherVMClient) InstanceCreate(i server.Instance, count int32) error {
	ic := server.InstanceCreate{
		Instance:  i,
		Instances: count,
	}

	buf, err := json.Marshal(ic)
	if err != nil {
		return err
	}

	resp, err := c.post("/v1/instances", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("InstanceCreate failed: %v", resp.Status)
	}
	return nil
}

func (c *RancherVMClient) InstanceGet(name string) (*vmapi.VirtualMachine, error) {
	resp, err := c.get("/v1/instances/" + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var vm vmapi.VirtualMachine
		if err := json.Unmarshal(buf, &vm); err != nil {
			return nil, err
		}
		return &vm, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("InstanceGet failed: %v", resp.Status)
}

func (c *RancherVMClient) InstanceList() ([]*vmapi.VirtualMachine, error) {
	resp, err := c.get("/v1/instances")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("InstanceList failed: %v", resp.Status)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list server.InstanceList
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}
	return list.Instances, nil
}

func (c *RancherVMClient) InstanceStop(name string) error {
	resp, err := c.post(fmt.Sprintf("/v1/instances/%s/stop", name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("InstanceStop failed: %v", resp.Status)
	}
	return nil
}

func (c *RancherVMClient) InstanceStart(name string) error {
	resp, err := c.post(fmt.Sprintf("/v1/instances/%s/start", name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("InstanceStart failed: %v", resp.Status)
	}
	return nil
}

func (c *RancherVMClient) InstanceUpdate(instance *vmapi.VirtualMachine) error {
	buf, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	resp, err := c.put("/v1/instances", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("InstanceUpdate failed: %v", resp.Status)
	}
	return nil
}

func (c *RancherVMClient) InstanceDelete(name string) error {
	resp, err := c.delete("/v1/instances/" + name)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("InstanceDelete failed: %v", resp.Status)
	}
	return nil
}
