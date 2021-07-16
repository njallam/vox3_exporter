package main

import (
	"encoding/csv"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	NAT_COL_TYPE       = 2
	NAT_COL_SRC_1      = 6
	NAT_COL_DST_1      = 7
	NAT_COL_SRC_PORT_1 = 8
	NAT_COL_DST_PORT_1 = 9
	NAT_COL_PACKETS_1  = 10
	NAT_COL_BYTES_1    = 11
	NAT_COL_PACKETS_2  = 16
	NAT_COL_BYTES_2    = 17
)

type NATMapping struct {
	Protocol        string
	Source          string
	SourcePort      int
	Destination     string
	DestinationPort int

	SentPackets     int
	SentBytes       int
	ReceivedPackets int
	ReceivedBytes   int
}

var localIPRegex = regexp.MustCompile(`^127\.|^192\.168`)

func (collector *Vox3Collector) FetchNAT() []NATMapping {
	data := collector.Fetch("/")
	token := csrfRegex.FindStringSubmatch(string(data))[1]
	if len(token) == 0 {
		log.Fatal("No CSRF token")
	}

	collector.Mutex.Lock()
	defer collector.Mutex.Unlock()
	response, err := collector.client.PostForm(collector.baseURL+"/modals/status-support/natMappingTable.lp", url.Values{
		"action":    {"downloadipv4nat"},
		"CSRFtoken": {token},
	})
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()
	r := csv.NewReader(response.Body)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		log.Println(err)
		return []NATMapping{}
	}

	mappings := make([]NATMapping, 0, len(rows))

	for _, row := range rows {
		protocol := row[NAT_COL_TYPE]
		if protocol != "tcp" && protocol != "udp" {
			continue
		}

		source := strings.Split(row[NAT_COL_SRC_1], "=")[1]
		sourcePort, _ := strconv.Atoi(strings.Split(row[NAT_COL_SRC_PORT_1], "=")[1])
		destination := strings.Split(row[NAT_COL_DST_1], "=")[1]
		destinationPort, _ := strconv.Atoi(strings.Split(row[NAT_COL_DST_PORT_1], "=")[1])
		sentPackets, _ := strconv.Atoi(strings.Split(row[NAT_COL_PACKETS_1], "=")[1])
		sentBytes, _ := strconv.Atoi(strings.Split(row[NAT_COL_BYTES_1], "=")[1])
		receivedPackets, _ := strconv.Atoi(strings.Split(row[NAT_COL_PACKETS_2], "=")[1])
		receivedBytes, _ := strconv.Atoi(strings.Split(row[NAT_COL_BYTES_2], "=")[1])

		if localIPRegex.MatchString(destination) || destinationPort == 53 {
			continue
		}

		mappings = append(mappings, NATMapping{
			protocol,
			source,
			sourcePort,
			destination,
			destinationPort,
			sentPackets,
			sentBytes,
			receivedPackets,
			receivedBytes,
		})
	}

	return mappings
}
