package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log.Println("Starting Vox3 Exporter")

	ip := os.Getenv("VOX3_IP")
	if len(ip) == 0 {
		ip = "192.168.1.1"
	}
	password := os.Getenv("VOX3_PASSWORD")
	if len(password) == 0 {
		log.Fatalf("Missing environment variable VOX3_PASSWORD")
	}

	prometheus.MustRegister(newVox3Collector(ip, os.Getenv("VOX3_PASSWORD")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Vox3 Exporter</title></head><body><h1>Vox3 exporter</h1><p><a href="metrics">Metrics</a></p></body></html>`))
	})
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :9917")
	log.Fatal(http.ListenAndServe(":9917", nil))
}
