package main

import (
	"log"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "vox3"

var uptimeRegex = regexp.MustCompile(`(?:(?:(?:(\d+) days?, )?(\d+) hours?, )(\d+) minutes? and )(\d+) seconds?`)
var speedRegex = regexp.MustCompile(`(\d+) kbps`)
var snrRegex = regexp.MustCompile(`([\d.]+) dB`)
var attenuationRegex = regexp.MustCompile(`(\w+) ([\d.]+) dB`)
var powerRegex = regexp.MustCompile(`([\d.]+) dBm`)
var delayRegex = regexp.MustCompile(`(\d+) ms`)

type Vox3Collector struct {
	baseURL  string
	password string

	client *http.Client
	sync.Mutex
}

var metrics = map[string]*prometheus.Desc{
	"dslUptime":    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "dsl_uptime_seconds"), "DSL uptime in seconds", nil, nil),
	"numberOfCuts": prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "number_of_cuts"), "Number of cuts", nil, nil),
	"currentRate":  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "current_rate_kbps"), "Current rate in kbps", []string{"direction"}, nil),
	"maximumRate":  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "maximum_rate_kbps"), "Maximum rate in kbps", []string{"direction"}, nil),
	"snr":          prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "snr_db"), "Signal-to-noise ratio in dB", []string{"direction"}, nil),
	"attenuation":  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "attenuation_db"), "Attenuation in dB", []string{"direction", "channel"}, nil),
	"power":        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "power_dbm"), "Power in dBm", []string{"direction"}, nil),
}

func newVox3Collector(ip string, password string) *Vox3Collector {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal("Could not create cookie jar")
	}
	client := &http.Client{
		Jar: jar,
	}
	return &Vox3Collector{
		baseURL:  "http://" + ip,
		password: password,
		client:   client,
	}
}

var columns = map[int]string{1: "downstream", 2: "upstream"}

func (collector *Vox3Collector) Collect(ch chan<- prometheus.Metric) {
	collector.Mutex.Lock()
	defer collector.Mutex.Unlock()
	response, err := collector.client.Get(collector.baseURL + "/modals/status-support/vdslStatus.lp")
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body", err)
	}
	if document.Find("#loginfrm").Length() > 0 {
		token, exists := document.Find("meta[name=CSRFtoken]").Attr("content")
		if !exists {
			log.Fatal("No CSRF token")
		}
		collector.login(token)
		response, err = collector.client.Get(collector.baseURL + "/modals/status-support/vdslStatus.lp")
		if err != nil {
			log.Fatal(err)
		}
		defer response.Body.Close()
		document, err = goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			log.Fatal("Error loading HTTP response body", err)
		}
	}

	uptimes := uptimeRegex.FindStringSubmatch(document.Find("#adslStat_info_uptime").Text())
	if len(uptimes) == 5 {
		days, _ := strconv.Atoi(uptimes[1])
		hours, _ := strconv.Atoi(uptimes[2])
		minutes, _ := strconv.Atoi(uptimes[3])
		seconds, _ := strconv.Atoi(uptimes[4])
		uptime := days*86400 + hours*3600 + minutes*60 + seconds
		ch <- prometheus.MustNewConstMetric(metrics["dslUptime"], prometheus.GaugeValue, float64(uptime))
	} else {
		log.Println("Failed to parse uptime")
	}

	cuts, err := strconv.Atoi(strings.TrimSpace(document.Find("#adslStat_info_cuts").Text()))
	if err == nil {
		ch <- prometheus.MustNewConstMetric(metrics["numberOfCuts"], prometheus.GaugeValue, float64(cuts))
	} else {
		log.Println("Failed to parse number of cuts")
	}

	document.Find("#line_quality tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td").Map(func(_ int, s *goquery.Selection) string {
			return s.Text()
		})
		if len(cells) != 3 {
			return
		}
		switch cells[0] {
		case "Current Rate":
			for i, v := range columns {
				value, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[i])[1])
				ch <- prometheus.MustNewConstMetric(metrics["currentRate"], prometheus.GaugeValue, float64(value), v)
			}
		case "Maximum Rate":
			for i, v := range columns {
				value, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[i])[1])
				ch <- prometheus.MustNewConstMetric(metrics["maximumRate"], prometheus.GaugeValue, float64(value), v)
			}
		case "Signal-to-Noise Ratio":
			for i, v := range columns {
				value, _ := strconv.ParseFloat(snrRegex.FindStringSubmatch(cells[i])[1], 64)
				ch <- prometheus.MustNewConstMetric(metrics["snr"], prometheus.GaugeValue, float64(value), v)
			}
		case "Attenuation":
			for i, v := range columns {
				entries := attenuationRegex.FindAllStringSubmatch(cells[i], -1)
				for _, entry := range entries {
					value, _ := strconv.ParseFloat(entry[2], 64)
					ch <- prometheus.MustNewConstMetric(metrics["attenuation"], prometheus.GaugeValue, value, v, entry[1])
				}
			}
		case "Power":
			for i, v := range columns {
				value, _ := strconv.ParseFloat(powerRegex.FindStringSubmatch(cells[i])[1], 64)
				ch <- prometheus.MustNewConstMetric(metrics["power"], prometheus.GaugeValue, value, v)
			}
		}

	})

}

func (collector *Vox3Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range metrics {
		ch <- desc
	}
}
