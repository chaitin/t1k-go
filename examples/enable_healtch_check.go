//go:build healthcheck
// +build healthcheck

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/chaitin/t1k-go"
)

func initDetect() *t1k.Server {
	server, err := t1k.NewWithPoolSize(os.Getenv("DETECTOR_ADDR"), 10)
	if err != nil {
		return nil
	}

	// enable health check
	hcConfig := &t1k.HealthCheckConfig{
		Interval:          2,
		HealthThreshold:   3,
		UnhealthThreshold: 5,
		Addresses:         []string{"1.1.1.1:8000", "1.1.1.2:8001"},
	}
	server.UpdateHealthCheckConfig(hcConfig)
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

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := server.HealthCheckStats()
		result := fmt.Sprintf("%#v\n\nIsHealth: %v\n", stats, server.IsHealth())
		goRoutineNum := runtime.NumGoroutine()
		result = fmt.Sprintf("%s\ngo routine num: %d\n", result, goRoutineNum)
		_, _ = w.Write([]byte(result))
	})
	_ = http.ListenAndServe(":80", nil)
}
