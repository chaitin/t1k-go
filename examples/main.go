package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/xbingW/t1k"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		d := t1k.NewDetector(t1k.Config{
			Addr: os.Getenv("DETECTOR_ADDR"),
		})
		res, err := d.DetectorRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !res.Allowed() {
			http.Error(w, fmt.Sprintf("blocked event id %s", res.EventID()), res.StatusCode())
			return
		}
		_, _ = w.Write([]byte("allowed"))
	})
	_ = http.ListenAndServe(":80", nil)
}
