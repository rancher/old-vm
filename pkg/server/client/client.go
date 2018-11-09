package client

import (
	"crypto/tls"
	"io"
	"net/http"
)

type RancherVMClient struct {
	client             *http.Client
	endpoint           string
	username, password string
}

func NewRancherVMClient(endpoint, username, password string, insecureSkipVerify bool) *RancherVMClient {
	var client *http.Client
	if insecureSkipVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	} else {
		client = http.DefaultClient
	}
	return &RancherVMClient{client, endpoint, username, password}
}

func (c *RancherVMClient) setBasicAuth(req *http.Request) {
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

func (c *RancherVMClient) do(req *http.Request) (*http.Response, error) {
	c.setBasicAuth(req)
	return c.client.Do(req)
}

func (c *RancherVMClient) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}

	return c.do(req)
}

func (c *RancherVMClient) post(path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.endpoint+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *RancherVMClient) put(path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.endpoint+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *RancherVMClient) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.endpoint+path, nil)
	if err != nil {
		return nil, err
	}

	return c.do(req)
}
