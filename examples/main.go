//go:build !healthcheck
// +build !healthcheck

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/chaitin/t1k-go"
)

func initDetect() *t1k.Server {
	server, err := t1k.NewWithPoolSize(os.Getenv("DETECTOR_ADDR"), 10)
	if err != nil {
		return nil
	}
	return server
}

func main() {
	server := initDetect()
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
	_ = http.ListenAndServe(":80", nil)
}
