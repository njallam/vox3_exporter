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
var snrRegex = regexp.MustCompile(`(\d+) db`)
var attenuationRegex = regexp.MustCompile(`(?:(\w+) (\w+))+`)
var powerRegex = regexp.MustCompile(`(\d+) dBm`)
var delayRegex = regexp.MustCompile(`(\d+) ms`)

type Vox3Collector struct {
	baseURL  string
	password string

	client *http.Client
	sync.Mutex
}

var metrics = map[string]*prometheus.Desc{
	"dslUptime":             prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "dsl_uptime_seconds"), "DSL uptime in seconds", nil, nil),
	"numberOfCuts":          prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "number_of_cuts"), "Number of cuts", nil, nil),
	"currentRateDownstream": prometheus.NewDesc(prometheus.BuildFQName(namespace, "downstream", "current_rate_kbps"), "Current downstream rate in kbps", nil, nil),
	"currentRateUpstream":   prometheus.NewDesc(prometheus.BuildFQName(namespace, "upstream", "current_rate_kbps"), "Current upstream rate in kbps", nil, nil),
	"maximumRateDownstream": prometheus.NewDesc(prometheus.BuildFQName(namespace, "downstream", "maximum_rate_kbps"), "Maximum downstream rate in kbps", nil, nil),
	"maximumRateUpstream":   prometheus.NewDesc(prometheus.BuildFQName(namespace, "upstream", "maximum_rate_kbps"), "Maximum upstream rate in kbps", nil, nil),
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
			downstream, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[1])[1])
			ch <- prometheus.MustNewConstMetric(metrics["currentRateDownstream"], prometheus.GaugeValue, float64(downstream))
			upstream, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[2])[1])
			ch <- prometheus.MustNewConstMetric(metrics["currentRateUpstream"], prometheus.GaugeValue, float64(upstream))
		case "Maximum Rate":
			downstream, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[1])[1])
			ch <- prometheus.MustNewConstMetric(metrics["maximumRateDownstream"], prometheus.GaugeValue, float64(downstream))
			upstream, _ := strconv.Atoi(speedRegex.FindStringSubmatch(cells[2])[1])
			ch <- prometheus.MustNewConstMetric(metrics["maximumRateUpstream"], prometheus.GaugeValue, float64(upstream))
		}
	})

}

func (collector *Vox3Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range metrics {
		ch <- desc
	}
}
