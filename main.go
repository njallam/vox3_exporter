package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "vox3"

var uptimeRegex = regexp.MustCompile(`(?:(?:(?:(\d+) days?, )?(\d+) hours?, )(\d+) minutes? and )(\d+) seconds?`)
var speedRegex = regexp.MustCompile(`(\d+) kbps`)
var snrRegex = regexp.MustCompile(`(\d+) db`)
var attenuationRegex = regexp.MustCompile(`(?:(\w+) (\w+))+`)
var powerRegex = regexp.MustCompile(`(\d+) dBm`)
var delayRegex = regexp.MustCompile(`(\d+) ms`)

type Vox3Collector struct {
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

func newVox3Collector() *Vox3Collector {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal("Could not create cookie jar")
	}
	client := &http.Client{
		Jar: jar,
	}

	return &Vox3Collector{
		client: client,
	}
}

func (collector *Vox3Collector) Collect(ch chan<- prometheus.Metric) {
	collector.Mutex.Lock()
	defer collector.Mutex.Unlock()
	response, err := collector.client.Get("http://192.168.1.1/modals/status-support/vdslStatus.lp")
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
		response, err = collector.client.Get("http://192.168.1.1/modals/status-support/vdslStatus.lp")
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
	days, _ := strconv.Atoi(uptimes[0])
	hours, _ := strconv.Atoi(uptimes[1])
	minutes, _ := strconv.Atoi(uptimes[2])
	seconds, _ := strconv.Atoi(uptimes[3])
	uptime := days*86400 + hours*3600 + minutes*60 + seconds
	ch <- prometheus.MustNewConstMetric(metrics["dslUptime"], prometheus.GaugeValue, float64(uptime))

	cuts, err := strconv.Atoi(strings.TrimSpace(document.Find("#adslStat_info_cuts").Text()))
	if err != nil {
		log.Println("Could not parse cuts")
	}
	ch <- prometheus.MustNewConstMetric(metrics["numberOfCuts"], prometheus.GaugeValue, float64(cuts))

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

var gen = big.NewInt(2)
var k, _ = big.NewInt(0).SetString("ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b855f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773bca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb694b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73", 16)
var C, _ = big.NewInt(0).SetString("05b9e8ef059c6b32ea59fc1d322d37f04aa30bae5aa9003b8321e21ddb04e300", 16)
var u = "4a76a9a2402bdd18123389b72ebbda50a30f65aedb90d7273130edea4b29cc4c"

func (collector *Vox3Collector) login(token string) {
	randBytes := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, randBytes)
	if err != nil {
		panic("Random source is broken!")
	}
	F := big.NewInt(0).SetBytes(randBytes)

	D := big.NewInt(0)
	D.Exp(gen, F, k)
	x := D.Text(16)

	response, err := collector.client.PostForm("http://192.168.1.1/authenticate", url.Values{
		"CSRFtoken": {token},
		"I":         {"vodafone"},
		"A":         {x},
	})
	if err != nil {
		log.Fatal("Error loading HTTP response body", err)
	}
	var body struct {
		S string `json:"s"`
		B string `json:"B"`
	}
	jsonData, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal(jsonData, &body)

	g, _ := big.NewInt(0).SetString(body.B, 16)

	q := 256
	var dBytes = D.Bytes()
	if len(dBytes) > q {
		dBytes = dBytes[1:]
	}

	var gBytes = g.Bytes()
	if len(gBytes) > q {
		gBytes = gBytes[1:]
	}

	temp := sha256.Sum256(append(dBytes, gBytes...))
	h := big.NewInt(0).SetBytes(temp[:])
	temp = sha256.Sum256([]byte("vodafone:password"))
	temp2, _ := hex.DecodeString(body.S + hex.EncodeToString(temp[:]))
	temp = sha256.Sum256(temp2)
	n := big.NewInt(0).SetBytes(temp[:])

	a := big.NewInt(0)
	a.Exp(gen, n, k)
	a.Mul(C, a)
	a.Mod(a, k)

	b := big.NewInt(0)
	b.Mul(h, n)
	b.Mod(b, k)
	b.Add(b, F)
	b.Mod(b, k)

	g.Sub(g, a)
	g.Mod(g, k)
	g.Exp(g, b, k)

	e := g.Text(16)
	if len(e)%2 == 1 {
		e = "0" + e
	}

	temp2, _ = hex.DecodeString(e)
	temp = sha256.Sum256(temp2)
	B := hex.EncodeToString(temp[:])
	temp = sha256.Sum256([]byte("vodafone"))
	e = hex.EncodeToString(temp[:])
	temp2, _ = hex.DecodeString(u + e + body.S + x + body.B + B)
	temp = sha256.Sum256(temp2)
	y := hex.EncodeToString(temp[:])

	temp2, _ = hex.DecodeString(x + y + B)
	temp = sha256.Sum256(temp2)
	v := hex.EncodeToString(temp[:])

	response, err = collector.client.PostForm("http://192.168.1.1/authenticate", url.Values{
		"CSRFtoken": {token},
		"M":         {y},
	})
	if err != nil {
		log.Fatal("Error loading HTTP response body", err)
	}
	var body2 struct {
		M string `json:"M"`
	}
	jsonData, _ = ioutil.ReadAll(response.Body)
	json.Unmarshal(jsonData, &body2)

	if v == body2.M {
		log.Println("Login successful")
	}
}

func main() {
	prometheus.MustRegister(newVox3Collector())

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :9906")
	log.Fatal(http.ListenAndServe(":9906", nil))
}
