package httpext

import (
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

type ctxKey int

const (
	// CtxKeyHTTP3RoundTripper is a context key for the HTTP3RoundTripper.
	CtxKeyHTTP3RoundTripper ctxKey = iota
)

// HTTP3Proto is the HTTP/3 protocol name.
const HTTP3Proto = "HTTP/3"

// HTTP3Metrics is a set of metrics for HTTP/3.
type HTTP3Metrics struct {
	HTTP3ReqDuration       *metrics.Metric
	HTTP3ReqSending        *metrics.Metric
	HTTP3ReqWaiting        *metrics.Metric
	HTTP3ReqReceiving      *metrics.Metric
	HTTP3ReqTLSHandshaking *metrics.Metric
	HTTP3ReqConnecting     *metrics.Metric
	HTTP3Reqs              *metrics.Metric
}

const (
	// HTTP3ReqDurationName is the name of the HTTP3ReqDuration metric.
	HTTP3ReqDurationName = "http3_req_duration"
	// HTTP3ReqSendingName is the name of the HTTP3ReqSending metric.
	HTTP3ReqSendingName = "http3_req_sending"
	// HTTP3ReqWaitingName is the name of the HTTP3ReqWaiting metric.
	HTTP3ReqWaitingName = "http3_req_waiting"
	// HTTP3ReqReceivingName is the name of the HTTP3ReqReceiving metric.
	HTTP3ReqReceivingName = "http3_req_receiving"
	// HTTP3ReqTLSHandshakingName is the name of the HTTP3ReqTLSHandshaking metric.
	HTTP3ReqTLSHandshakingName = "http3_req_tls_handshaking"
	// HTTP3ReqConnectingName is the name of the HTTP3ReqConnecting metric.
	HTTP3ReqConnectingName = "http3_req_connecting"
	// HTTP3ReqsName is the name of the HTTP3Reqs metric.
	HTTP3ReqsName = "http3_reqs"
)

// RegisterMetrics registers the HTTP3Metrics.
func RegisterMetrics(vu modules.VU) (*HTTP3Metrics, error) {
	var err error
	registry := vu.InitEnv().Registry
	m := &HTTP3Metrics{}

	m.HTTP3ReqDuration, err = registry.NewMetric(HTTP3ReqDurationName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}
	m.HTTP3ReqReceiving, err = registry.NewMetric(HTTP3ReqReceivingName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}
	m.HTTP3ReqWaiting, err = registry.NewMetric(HTTP3ReqWaitingName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}
	m.HTTP3ReqSending, err = registry.NewMetric(HTTP3ReqSendingName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}
	m.HTTP3Reqs, err = registry.NewMetric(HTTP3ReqsName, metrics.Counter)
	if err != nil {
		return m, err
	}
	m.HTTP3ReqTLSHandshaking, err = registry.NewMetric(HTTP3ReqTLSHandshakingName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}
	m.HTTP3ReqConnecting, err = registry.NewMetric(HTTP3ReqConnectingName, metrics.Trend, metrics.Time)
	if err != nil {
		return m, err
	}

	return m, nil
}
