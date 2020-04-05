package prometheus

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/cloudflare/cfssl/log"
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/api/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

const (
	namespace = "k6"
)

var (
	metrics = make([]v1.Metric, 0)
)

func HandlePrometheusMetrics() http.Handler {
	exporter := NewExporter()
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("k6_exporter"))
	prom := promhttp.Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engine := common.GetEngine(r.Context())
		var t time.Duration
		if engine.Executor != nil {
			t = engine.Executor.GetTime()
		}

		for _, m := range engine.Metrics {
			metrics = append(metrics, v1.NewMetric(m, t))
		}
		prom.ServeHTTP(w, r)
	})
}

//prometheus exporter
type Exporter struct {
	mutex  sync.Mutex
	client *http.Client

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
func NewExporter() *Exporter {
	return &Exporter{
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

// Collect fetches the stats from configured location and delivers them
// as Prometheus metrics.
// It implements prometheus.Collector.

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	for _, d := range metrics {
		if d.Name == "vus" {
			ch <- prometheus.MustNewConstMetric(e.vus, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "iterations" {
			ch <- prometheus.MustNewConstMetric(e.iterations, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "checks" {
			ch <- prometheus.MustNewConstMetric(e.checks, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "http_reqs" {
			ch <- prometheus.MustNewConstMetric(e.httpReqs, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "http_req_blocked" {
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric	(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqBlocked, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		} else if d.Name == "data_sent" {
			ch <- prometheus.MustNewConstMetric(e.dataSent, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "data_received" {
			ch <- prometheus.MustNewConstMetric(e.dataReceived, prometheus.GaugeValue, float64(d.Sample["value"]))
		} else if d.Name == "http_req_connecting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqConnecting, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		} else if d.Name == "http_req_sending" {
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqSending, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		} else if d.Name == "http_req_waiting" {
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqWaiting, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		} else if d.Name == "http_req_receiving" {
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqReceiving, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		} else if d.Name == "http_req_duration" {
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["min"], "min")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["max"], "max")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["avg"], "avg")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["p(90)"], "p(90)")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["p(95)"], "p(95)")
			ch <- prometheus.MustNewConstMetric(e.httpReqDuration, prometheus.GaugeValue, d.Sample["p(99)"], "p(99)")
		}
	}
	metrics = make([]v1.Metric, 0)
	return nil
}