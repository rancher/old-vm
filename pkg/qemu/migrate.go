package qemu

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang/glog"
)

func (c *MonitorClient) Migrate(uri string) error {
	c.migrate(uri)

	start := time.Now()
	t := time.NewTicker(1 * time.Second)
	for _ = range t.C {
		// Silently read any events that may be written. We expect a STOP event
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

		elapsedTime := time.Now().Sub(start)
		switch status {
		case "failed":
			return errors.New(fmt.Sprintf("Migration failed: %v", reply))
		case "active":
			ramMap, ok := replyMap["ram"].(map[string]interface{})
			if !ok {
				return errors.New(fmt.Sprintf("Unrecognized reply: %v", reply))
			}

			transferred := ramMap["transferred"].(float64)
			total := ramMap["total"].(float64)
			mbps := ramMap["mbps"].(float64)

			remainingTime := time.Duration(math.MaxInt64)
			if transferred > 0.0 {
				// Instantaneous computation is good enuff
				remainingTime = time.Duration(float64(elapsedTime)*total/transferred - float64(elapsedTime))
			}
			glog.Infof("status=%s\telapsed=%v\tremaining=%v\tmbps=%v", status, elapsedTime, remainingTime, mbps)

		case "completed":
			glog.Infof("status=%s total=%v", status, elapsedTime)
			return nil
		default:
			continue
		}
	}

	t.Stop()
	return nil
}
