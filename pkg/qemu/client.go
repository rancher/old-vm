package qemu

import (
	"net"
	"time"

	"github.com/golang/glog"
)

type MonitorClient struct {
	path string
	conn net.Conn
}

func NewMonitorClient(path string) *MonitorClient {
	c := &MonitorClient{
		path: path,
	}

	var err error
	c.conn, err = net.DialTimeout("unix", c.path, 5*time.Second)
	if err != nil {
		panic(err)
	}

	reply, err := c.readReply()
	if err != nil {
		panic(err)
	}
	glog.Infof("connected: %s", string(reply))

	c.setCapabilities()

	return c
}
