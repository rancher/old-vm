package server

import (
	// "bytes"
	"net/http"
	"reflect"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	keepAlivePeriod = 60 * time.Second

	writeWait = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewStreamHandlerFunc(watcher *Watcher, listFunc func() (interface{}, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Warningf("stream: %s", err.Error())
			return
		}
		glog.V(3).Info("websocket: open")

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					glog.V(3).Info(err.Error())
					return
				}
			}
		}()

		resp, err := writeList(conn, nil, listFunc)
		if err != nil {
			glog.Warningf("stream: %s", err.Error())
			return
		}

		rateLimitTicker := maybeNewTicker(getPeriod(r))
		keepAliveTicker := time.NewTicker(keepAlivePeriod)
		for {
			if rateLimitTicker != nil {
				<-rateLimitTicker.C
			}
			select {
			case <-done:
				return
			case <-watcher.Events():
				resp, err = writeList(conn, resp, listFunc)
			case <-keepAliveTicker.C:
				err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
			}
			if err != nil {
				glog.Warningf("stream: %s", err.Error())
				return
			}
		}
	}
}

func writeList(conn *websocket.Conn, oldResp interface{}, listFunc func() (interface{}, error)) (interface{}, error) {
	newResp, err := listFunc()
	if err != nil {
		return oldResp, err
	}

	if oldResp != nil && reflect.DeepEqual(oldResp, newResp) {
		return oldResp, nil
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err = conn.WriteJSON(newResp); err != nil {
		return oldResp, err
	}

	return newResp, nil
}

func maybeNewTicker(d time.Duration) *time.Ticker {
	var ticker *time.Ticker
	if d > 0*time.Second {
		ticker = time.NewTicker(d)
	}
	return ticker
}

func getPeriod(r *http.Request) time.Duration {
	period := 0 * time.Second
	periodString := mux.Vars(r)["period"]
	if periodString != "" {
		period, _ = time.ParseDuration(periodString)
	}
	switch {
	case period < 0*time.Second:
		period = 0 * time.Second
	case period > 30*time.Second:
		period = 30 * time.Second
	}
	return period
}
