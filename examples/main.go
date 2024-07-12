package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/xbingW/t1k-go"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		server, err := t1k.NewWithPoolSize(os.Getenv("DETECTOR_ADDR"), 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res, err := server.DetectHttpRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !res.Blocked() {
			http.Error(w, fmt.Sprintf("blocked event id %s", res.EventID()), res.StatusCode())
			return
		}
		_, _ = w.Write([]byte("allowed"))
	})
	_ = http.ListenAndServe(":80", nil)
}
