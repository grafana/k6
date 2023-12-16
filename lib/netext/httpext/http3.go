package httpext

import (
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

type ctxKey int

const (
	CtxKeyHTTP3RoundTripper ctxKey = iota
)

const HTTP3Proto = "HTTP/3"

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
	HTTP3ReqDurationName       = "http3_req_duration"
	HTTP3ReqSendingName        = "http3_req_sending"
	HTTP3ReqWaitingName        = "http3_req_waiting"
	HTTP3ReqReceivingName      = "http3_req_receiving"
	HTTP3ReqTLSHandshakingName = "http3_req_tls_handshaking"
	HTTP3ReqConnectingName     = "http3_req_connecting"
	HTTP3ReqsName              = "http3_reqs"
)

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
