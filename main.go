package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	prometheus.MustRegister(newVox3Collector(os.Getenv("VOX3_IP"), os.Getenv("VOX3_PASSWORD")))

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :9906")
	log.Fatal(http.ListenAndServe(":9906", nil))
}
