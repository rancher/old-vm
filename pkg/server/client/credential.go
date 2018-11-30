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

func (c *RancherVMClient) CredentialCreate(name, publicKey string) error {
	cc := server.CredentialCreate{
		Name:      name,
		PublicKey: publicKey,
	}

	buf, err := json.Marshal(cc)
	if err != nil {
		return err
	}

	resp, err := c.post("/v1/credential", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("CredentialCreate failed: %v", resp.Status)
	}
	return nil
}

func (c *RancherVMClient) CredentialGet(name string) (*vmapi.Credential, error) {
	resp, err := c.get("/v1/credential/" + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var cred vmapi.Credential
		if err := json.Unmarshal(buf, &cred); err != nil {
			return nil, err
		}
		return &cred, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("CredentialGet failed: %v", resp.Status)
}

func (c *RancherVMClient) CredentialDelete(name string) error {
	resp, err := c.delete("/v1/credential/" + name)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("CredentialDelete failed: %v", resp.Status)
	}
	defer resp.Body.Close()

	return nil
}
