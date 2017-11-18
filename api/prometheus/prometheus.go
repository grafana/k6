package prometheus

import (
	"net/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"

	log "github.com/sirupsen/logrus"
	"sync"
	"encoding/json"
	"fmt"
	"crypto/tls"
	"io/ioutil"
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

	up         *prometheus.Desc
	vus        *prometheus.Desc
	vusMax     *prometheus.Desc
	iterations *prometheus.Desc

	httpReqs *prometheus.Desc
	
	httpReqBlocked_min *prometheus.Desc
	httpReqBlocked_max *prometheus.Desc
	httpReqBlocked_avg *prometheus.Desc
	httpReqBlocked_90p *prometheus.Desc
	httpReqBlocked_95p *prometheus.Desc

	httpReqLookingUp *prometheus.Desc

	httpReqConnecting_min *prometheus.Desc
	httpReqConnecting_max *prometheus.Desc
	httpReqConnecting_avg *prometheus.Desc
	httpReqConnecting_90p *prometheus.Desc
	httpReqConnecting_95p *prometheus.Desc

	httpReqSending_min *prometheus.Desc
	httpReqSending_max *prometheus.Desc
	httpReqSending_avg *prometheus.Desc
	httpReqSending_90p *prometheus.Desc
	httpReqSending_95p *prometheus.Desc

	httpReqWaiting_min *prometheus.Desc
	httpReqWaiting_max *prometheus.Desc
	httpReqWaiting_avg *prometheus.Desc
	httpReqWaiting_90p *prometheus.Desc
	httpReqWaiting_95p *prometheus.Desc

	httpReqReceiving_min *prometheus.Desc
	httpReqReceiving_max *prometheus.Desc
	httpReqReceiving_avg *prometheus.Desc
	httpReqReceiving_90p *prometheus.Desc
	httpReqReceiving_95p *prometheus.Desc

	httpReqDuration_min *prometheus.Desc
	httpReqDuration_max *prometheus.Desc
	httpReqDuration_avg *prometheus.Desc
	httpReqDuration_90p *prometheus.Desc
	httpReqDuration_95p *prometheus.Desc
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
		httpReqs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_total"),
			"How many HTTP requests has k6 generated, in total",
			nil,
			nil),
		httpReqBlocked_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_blocked_min_total"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			nil,
			nil),
		httpReqBlocked_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_blocked_max_total"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			nil,
			nil),
		httpReqBlocked_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_blocked_avg_total"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			nil,
			nil),
		httpReqBlocked_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_blocked_90p_total"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			nil,
			nil),
		httpReqBlocked_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_blocked_95p_total"),
			"Time (ms) spent blocked (waiting for a free TCP connection slot) before initiating request.",
			nil,
			nil),
		httpReqLookingUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_looking_up_total"),
			"Time (ms) spent looking up remote host name in DNS",
			[]string{"min", "max", "avg", "percentiles"},
			nil),
		httpReqConnecting_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_connecting_min_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqConnecting_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_connecting_max_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqConnecting_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_connecting_avg_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqConnecting_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_connecting_90p_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqConnecting_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_reqs_connecting_95p_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqSending_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_min_total"),
			"Time (ms) spent sending data to remote host",
			nil,
			nil),
		httpReqSending_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_max_total"),
			"Time (ms) spent sending data to remote host",
			nil,
			nil),
		httpReqSending_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_avg_total"),
			"Time (ms) spent sending data to remote host",
			nil,
			nil),
		httpReqSending_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_90p_total"),
			"Time (ms) spent sending data to remote host",
			nil,
			nil),
		httpReqSending_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_sending_95p_total"),
			"Time (ms) spent sending data to remote host",
			nil,
			nil),
		httpReqWaiting_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_min_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			nil,
			nil),
		httpReqWaiting_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_max_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			nil,
			nil),
		httpReqWaiting_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_avg_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			nil,
			nil),
		httpReqWaiting_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_90p_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			nil,
			nil),
		httpReqWaiting_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_waiting_95p_total"),
			"Time (ms) spent waiting for response from remote host (a.k.a. 'time to first byte', or 'TTFB')",
			nil,
			nil),
		httpReqReceiving_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_min_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqReceiving_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_max_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqReceiving_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_avg_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqReceiving_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_90p_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqReceiving_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_receiving_95p_total"),
			"Time (ms) spent establishing TCP connection to remote host",
			nil,
			nil),
		httpReqDuration_min: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_min_total"),
			"Total time (ms) for request, excluding time spent blocked (http_req_blocked), DNS lookup (http_req_looking_up) and TCP connect (http_req_connecting) time",
			nil,
			nil),
		httpReqDuration_max: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_max_total"),
			"Total time (ms) for request, excluding time spent blocked (http_req_blocked), DNS lookup (http_req_looking_up) and TCP connect (http_req_connecting) time",
			nil,
			nil),
		httpReqDuration_avg: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_avg_total"),
			"Total time (ms) for request, excluding time spent blocked (http_req_blocked), DNS lookup (http_req_looking_up) and TCP connect (http_req_connecting) time",
			nil,
			nil),
		httpReqDuration_90p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_90p_total"),
			"Total time (ms) for request, excluding time spent blocked (http_req_blocked), DNS lookup (http_req_looking_up) and TCP connect (http_req_connecting) time",
			nil,
			nil),
		httpReqDuration_95p: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "http_req_duration_95p_total"),
			"Total time (ms) for request, excluding time spent blocked (http_req_blocked), DNS lookup (http_req_looking_up) and TCP connect (http_req_connecting) time",
			nil,
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
	ch <- e.httpReqs
	ch <- e.httpReqBlocked_min
	ch <- e.httpReqBlocked_max
	ch <- e.httpReqBlocked_avg
	ch <- e.httpReqBlocked_90p
	ch <- e.httpReqBlocked_95p

	ch <- e.httpReqLookingUp

	ch <- e.httpReqConnecting_min
	ch <- e.httpReqConnecting_max
	ch <- e.httpReqConnecting_avg
	ch <- e.httpReqConnecting_90p
	ch <- e.httpReqConnecting_95p

	ch <- e.httpReqSending_min
	ch <- e.httpReqSending_max
	ch <- e.httpReqSending_avg
	ch <- e.httpReqSending_90p
	ch <- e.httpReqSending_95p

	ch <- e.httpReqWaiting_min
	ch <- e.httpReqWaiting_max
	ch <- e.httpReqWaiting_avg
	ch <- e.httpReqWaiting_90p
	ch <- e.httpReqWaiting_95p

	ch <- e.httpReqReceiving_min
	ch <- e.httpReqReceiving_max
	ch <- e.httpReqReceiving_avg
	ch <- e.httpReqReceiving_90p
	ch <- e.httpReqReceiving_95p

	ch <- e.httpReqDuration_min
	ch <- e.httpReqDuration_max
	ch <- e.httpReqDuration_avg
	ch <- e.httpReqDuration_90p
	ch <- e.httpReqDuration_95p
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
	return
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
	resp.Body.Close()
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
		} else if d.ID == "http_reqs" {
			ch <- prometheus.MustNewConstMetric(e.httpReqs, prometheus.GaugeValue, float64(d.Attributes.Sample.Value))
		} else if d.ID == "http_req_blocked" {
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked_max, prometheus.GaugeValue, d.Attributes.Sample.Max)
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		} else if d.ID == "http_req_connecting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting_max, prometheus.GaugeValue, d.Attributes.Sample.Max)
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		} else if d.ID == "http_req_sending" {
			ch <- prometheus.MustNewConstMetric(e.httpReqSending_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqSending_max, prometheus.GaugeValue, d.Attributes.Sample.Max)
			ch <- prometheus.MustNewConstMetric(e.httpReqSending_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqSending_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqSending_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		} else if d.ID == "http_req_waiting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting_max, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		} else if d.ID == "http_req_receiving" {
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving_max, prometheus.GaugeValue, d.Attributes.Sample.Max)
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		} else if d.ID == "http_req_duration" {
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration_min, prometheus.GaugeValue, d.Attributes.Sample.Min)
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration_max, prometheus.GaugeValue, d.Attributes.Sample.Max)
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration_avg, prometheus.GaugeValue, d.Attributes.Sample.Avg)
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration_90p, prometheus.GaugeValue, d.Attributes.Sample.P90)
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration_95p, prometheus.GaugeValue, d.Attributes.Sample.P95)
		}

	}

	return nil
}
