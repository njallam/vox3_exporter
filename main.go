package main

import (
	"html/template"
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
	natTableEnabled := len(os.Getenv("VOX3_NAT_TABLE")) > 0

	collector := newVox3Collector(ip, password)

	prometheus.MustRegister(collector)

	templates := template.Must(template.ParseFiles("nat.html"))

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(`<html><head><title>Vox3 Exporter</title></head>` +
			`<body><h1>Vox3 exporter</h1>` +
			`<p><a href="metrics">Metrics</a></p>` +
			`<p><a href="nat">NAT Table</a></p>` +
			`</body></html>`))
	})
	http.Handle("/metrics", promhttp.Handler())

	if natTableEnabled {
		http.HandleFunc("/nat", func(rw http.ResponseWriter, r *http.Request) {
			table := collector.FetchNAT()
			templates.Execute(rw, table)
		})
	}

	log.Println("Beginning to serve on port :9917")
	log.Fatal(http.ListenAndServe(":9917", nil))
}
