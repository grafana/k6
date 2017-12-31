package prometheus

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"sync"
)

const (
	namespace = "k6"
)

var (
	scrapeURI = "http://localhost:6565/v1/metrics"
)

func HandlePrometheusMetrics() http.Handler {
	exporter := NewExporter(scrapeURI)
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("k6_exporter"))
	return promhttp.Handler()
}

//prometheus exporter
type Exporter struct {
	URI    string
	mutex  sync.Mutex
	client *http.Client

	up                *prometheus.Desc
	vus               *prometheus.Desc
	vusMax            *prometheus.Desc
	iterations        *prometheus.Desc
	checks            *prometheus.Desc
	httpReqs          *prometheus.Desc
	httpReqBlocked    *prometheus.Desc
	dataSent          *prometheus.Desc
	dataReceived      *prometheus.Desc
	httpReqConnecting *prometheus.Desc
	httpReqSending    *prometheus.Desc
	httpReqWaiting    *prometheus.Desc
	httpReqReceiving  *prometheus.Desc
	httpReqDuration   *prometheus.Desc
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string) *Exporter {
	return &Exporter{
		URI: uri,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could k6 be reached",
			nil,
			nil),
		vus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "vus_length"),
			"Current number of active virtual users",
			nil,
			nil),
		vusMax: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "vus_max_length"),
			"Max possible number of virtual users",
			nil,
			nil),
		iterations: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "iterations_total"),
			"The aggregate number of times the VUs in the test have executed the JS script",
			nil,
			nil),
		checks: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_checks_total"),
			"Number of different checks",
			nil,
			nil),
		httpReqs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_total"),
			"How many HTTP requests has k6 generated, in total",
			nil,
			nil),
		httpReqBlocked: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_blocked"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			[]string{"type"},
			nil),
		dataSent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "data_sent_total"),
			"Data sent in bytes",
			nil,
			nil),
		dataReceived: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "data_received_total"),
			"Data received in bytes",
			nil,
			nil),
		httpReqConnecting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_connecting_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			[]string{"type"},
			nil),
		httpReqSending: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_total"),
			"Time (ms) spent sending data to remote host",
			[]string{"type"},
			nil),
		httpReqWaiting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			[]string{"type"},
			nil),
		httpReqReceiving: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			[]string{"type"},
			nil),
		httpReqDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_total"),
			"Total time (ms) for request, excluding time spent blocked, DNS lookup and TCP connect time",
			[]string{"type"},
			nil),
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			},
		},
	}
}

// Describe describes all the metrics ever exported by the k6 exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.vus
	ch <- e.vusMax
	ch <- e.iterations
	ch <- e.checks
	ch <- e.httpReqs
	ch <- e.httpReqBlocked
	ch <- e.dataSent
	ch <- e.dataReceived
	ch <- e.httpReqConnecting
	ch <- e.httpReqSending
	ch <- e.httpReqWaiting
	ch <- e.httpReqReceiving
	ch <- e.httpReqDuration
}

// Collect fetches the stats from configured k6 location and delivers them
// as Prometheus metrics.
// It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping k6: %s", err)
	}
}

// json data structure for k6 metrics api
type jsonData struct {
	Data []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Type     string      `json:"type"`
			Contains string      `json:"contains"`
			Tainted  interface{} `json:"tainted"`
			Sample   struct {
				Value int     `json:"value"`
				Avg   float64 `json:"avg"`
				Max   float64 `json:"max"`
				Med   int     `json:"med"`
				Min   float64 `json:"min"`
				P90   float64 `json:"p(90)"`
				P95   float64 `json:"p(95)"`
			} `json:"sample"`
		} `json:"attributes"`
	} `json:"data"`
}

// Collect fetches the stats from configured location and delivers them
// as Prometheus metrics.
// It implements prometheus.Collector.

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {

	resp, err := e.client.Get(e.URI)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error scraping k6 api: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	// get data from body of response and check if there was a read error
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	// close connection
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// check http code
	if resp.StatusCode != 200 {
		fmt.Println("There was an error")
		return fmt.Errorf("Status %s (%d)", resp.Status, resp.StatusCode)
	}

	//init struct for unmarshal and check that there was no unmarshalling error
	jdata := jsonData{}
	jError := json.Unmarshal(body, &jdata)
	if jError != nil {
		log.Fatal(jError)
	}

	for _, d := range jdata.Data {
		if d.ID == "vus" {
			ch <- prometheus.MustNewConstMetric(e.vus, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "iterations" {
			ch <- prometheus.MustNewConstMetric(e.iterations, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "checks" {
			ch <- prometheus.MustNewConstMetric(e.checks, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "http_reqs" {
			ch <- prometheus.MustNewConstMetric(e.httpReqs, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "http_req_blocked" {
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Attributes.Sample.Max, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		} else if d.ID == "data_sent" {
			ch <- prometheus.MustNewConstMetric(e.dataSent, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "data_received" {
			ch <- prometheus.MustNewConstMetric(e.dataReceived, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "http_req_connecting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Attributes.Sample.Max, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		} else if d.ID == "http_req_sending" {
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Attributes.Sample.Max, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		} else if d.ID == "http_req_waiting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Attributes.Sample.Min, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		} else if d.ID == "http_req_receiving" {
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Attributes.Sample.Max, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		} else if d.ID == "http_req_duration" {
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Attributes.Sample.Min, "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Attributes.Sample.Max, "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Attributes.Sample.Avg, "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Attributes.Sample.P90, "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Attributes.Sample.P95, "p(95)")
		}

	}

	return nil
}
