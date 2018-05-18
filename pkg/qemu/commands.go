package qemu

import (
	"encoding/json"
	"net"
	"time"

	"github.com/golang/glog"
)

func (c *MonitorClient) migrate(uri string) {
	cmd := monitorMakeCommandMigrate(uri, true)
	c.monitorExecCommand(cmd)
}

func (c *MonitorClient) queryMigrate() interface{} {
	cmd := monitorMakeCommandQueryMigrate()
	return c.monitorExecCommand(cmd)
}

func (c *MonitorClient) setCapabilities() {
	cmd := monitorMakeCommandCapabilities()
	c.monitorExecCommand(cmd)
}

func monitorMakeCommandMigrate(uri string, detach bool) []byte {
	return monitorMakeCommand("migrate", map[string]interface{}{
		"uri":    uri,
		"detach": detach,
	})
}

func monitorMakeCommandQueryMigrate() []byte {
	return monitorMakeCommand("query-migrate", nil)
}

func monitorMakeCommandCapabilities() []byte {
	return monitorMakeCommand("qmp_capabilities", nil)
}

func monitorMakeCommand(execute string, arguments map[string]interface{}) []byte {
	v := map[string]interface{}{
		"execute": execute,
	}
	if arguments != nil {
		v["arguments"] = arguments
	}

	m, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return m
}

func (c *MonitorClient) monitorExecCommand(cmd []byte) interface{} {
	glog.V(5).Infof("cmd: %s", string(cmd))
	c.conn.Write(cmd)

	rawReply, err := c.readReply()
	if err != nil {
		panic(err)
	}
	glog.V(5).Infof("rawReply: %s", string(rawReply))

	reply := parseReply(rawReply)
	glog.V(5).Infof("reply: %+v", reply)
	return reply
}

func parseReply(rawReply []byte) interface{} {
	var reply map[string]interface{}

	err := json.Unmarshal(rawReply, &reply)
	if err != nil {
		panic(err)
	}

	return reply["return"]
}

func (c *MonitorClient) readReply() ([]byte, error) {
	var reply []byte
	for i := 512; ; i = i << 1 {
		buf := make([]byte, i)

		n, err := c.conn.Read(buf)
		if err != nil {
			return []byte{}, err
		}
		reply = append(reply, buf[:n]...)

		if n < i {
			break
		}
	}
	return reply, nil
}

func (c *MonitorClient) readSilently() {
	c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	defer c.conn.SetReadDeadline(time.Now().Add(42 * time.Hour))

	if _, err := c.readReply(); err != nil {
		switch err.(type) {
		case *net.OpError:
			return
		default:
			panic(err)
		}
	}
}
