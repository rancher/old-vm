package qemu

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"
)

func (c *MonitorClient) Migrate(uri string) error {
	c.migrate(true, uri)
	t := time.NewTicker(1 * time.Second)

	for _ = range t.C {
		// Silently consume any events that may be written. We expect a STOP event
		// Example:
		// {"timestamp": {"seconds": 1525980624, "microseconds": 368241}, "event": "STOP"}
		c.readSilently()

		reply := c.queryMigrate()
		replyMap, ok := reply.(map[string]interface{})
		if !ok {
			glog.Warning("Didn't get a reply, maybe we neglected an event?")
			continue
		}

		status, ok := replyMap["status"]
		if !ok {
			return errors.New(fmt.Sprintf("Unrecognized reply: %v", reply))
		}

		switch status {
		case "failed":
			return errors.New(fmt.Sprintf("Migration failed: %v", reply))
		// TODO break once succeeded/failed
		case "active":
			// TODO display some nice metrics
			// map[status:active setup-time:191 total-time:15002 ram:map[dirty-sync-count:1 transferred:4.9483918e+08 dirty-pages-rate:0 skipped:0 normal-bytes:4.93858816e+08 total:5.54508288e+08 mbps:176.694842 duplicate:1613 normal:120571 remaining:5.4038528e+07] expected-downtime:300]
		case "completed":
			return nil
		default:
			continue
		}
	}

	t.Stop()
	return nil
}
