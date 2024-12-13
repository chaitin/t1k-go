//go:build new_connection_pool
// +build new_connection_pool

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/chaitin/t1k-go"
)

func initDetect_1(addr string) *t1k.ChannelPool {
	pc := &t1k.PoolConfig{
		InitialCap:  1,
		MaxIdle:     16,
		MaxCap:      32,
		Factory:     &t1k.TcpFactory{Addr: addr},
		IdleTimeout: 30 * time.Second,
	}
	server, err := t1k.NewChannelPool(pc)
	if err != nil {
		return nil
	}
	return server
}

func main() {
	server := initDetect_1(os.Getenv("DETECTOR_ADDR"))
	if server == nil {
		fmt.Println("Init detect error")
		return
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := server.DetectHttpRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if res.Blocked() {
			http.Error(w, fmt.Sprintf("blocked event id %s", res.EventID()), res.StatusCode())
			return
		}
		_, _ = w.Write([]byte("allowed"))
	})
	_ = http.ListenAndServe(":8000", nil)
}
