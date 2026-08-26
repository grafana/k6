// Package http implements a proxy that applies disruptions to HTTP requests
package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
)

// Disruption specifies disruptions in http requests
type Disruption struct {
	// Average delay introduced to requests
	AverageDelay time.Duration
	// Variation in the delay (with respect of the average delay)
	DelayVariation time.Duration
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Error code to be returned by requests selected in the error rate
	ErrorCode int
	// Body to be returned when an error is injected
	ErrorBody string
	// List of url paths to be excluded from disruptions
	Excluded []string
}

// Proxy defines the parameters used by the proxy for processing http requests and its execution state
type proxy struct {
	listener   net.Listener
	disruption Disruption
	srv        *http.Server
	metrics    *protocol.MetricMap
}

// NewProxy return a new Proxy for HTTP requests
func NewProxy(listener net.Listener, upstreamAddress string, d Disruption) (protocol.Proxy, error) {
	if upstreamAddress == "" {
		return nil, fmt.Errorf("proxy's forwarding address must be provided")
	}

	if d.DelayVariation > d.AverageDelay {
		return nil, fmt.Errorf("variation must be less that average delay")
	}

	if d.ErrorRate < 0.0 || d.ErrorRate > 1.0 {
		return nil, fmt.Errorf("error rate must be in the range [0.0, 1.0]")
	}

	if d.ErrorRate > 0.0 && d.ErrorCode == 0 {
		return nil, fmt.Errorf("error code must be a valid http error code")
	}

	upstreamURL, err := url.Parse(upstreamAddress)
	if err != nil {
		return nil, err
	}

	metrics := protocol.NewMetricMap(supportedMetrics()...)

	handler := &httpHandler{
		upstreamURL: *upstreamURL,
		disruption:  d,
		metrics:     metrics,
	}

	return &proxy{
		listener:   listener,
		disruption: d,
		metrics:    metrics,
		srv: &http.Server{
			Handler: handler,
		},
	}, nil
}

// httpHandler implements a http.Handler for disrupting request to a upstream server
type httpHandler struct {
	upstreamURL url.URL
	disruption  Disruption
	metrics     *protocol.MetricMap
}

// isExcluded checks whether a request should be proxied through without any kind of modification whatsoever.
func (h *httpHandler) isExcluded(r *http.Request) bool {
	for _, excluded := range h.disruption.Excluded {
		if strings.EqualFold(r.URL.Path, excluded) {
			return true
		}
	}

	return false
}

// forward forwards a request to the upstream URL.
// Request is performed immediately, but response won't be sent before the duration specified in delay.
func (h *httpHandler) forward(rw http.ResponseWriter, req *http.Request, delay time.Duration) {
	timer := time.After(delay)

	upstreamReq := req.Clone(context.Background())
	upstreamReq.Host = h.upstreamURL.Host
	upstreamReq.URL.Host = h.upstreamURL.Host
	upstreamReq.URL.Scheme = h.upstreamURL.Scheme
	upstreamReq.RequestURI = "" // It is an error to set this field in an HTTP client request.

	response, err := http.DefaultClient.Do(upstreamReq)
	<-timer
	if err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprint(rw, err)
		return
	}

	defer func() {
		// Fully consume and then close upstream response body.
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	// Mirror headers.
	for key, values := range response.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}

	// Mirror status code.
	rw.WriteHeader(response.StatusCode)

	// ignore errors writing body, nothing to do.
	_, _ = io.Copy(rw, response.Body)
}

// injectError waits sleeps the duration specified in delay and then writes the configured error downstream.
func (h *httpHandler) injectError(rw http.ResponseWriter, delay time.Duration) {
	time.Sleep(delay)

	rw.WriteHeader(h.disruption.ErrorCode)
	_, _ = rw.Write([]byte(h.disruption.ErrorBody))
}

func (h *httpHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.metrics.Inc(protocol.MetricRequests)

	if h.isExcluded(req) {
		h.metrics.Inc(protocol.MetricRequestsExcluded)
		//nolint:contextcheck // Unclear which context the linter requires us to propagate here.
		h.forward(rw, req, 0)
		return
	}

	delay := h.disruption.AverageDelay
	if h.disruption.DelayVariation > 0 {
		variation := int64(h.disruption.DelayVariation)
		delay += time.Duration(variation - 2*rand.Int63n(variation))
	}

	if h.disruption.ErrorRate > 0 && rand.Float32() <= h.disruption.ErrorRate {
		h.metrics.Inc(protocol.MetricRequestsDisrupted)
		h.injectError(rw, delay)
		return
	}

	//nolint:contextcheck // Unclear which context the linter requires us to propagate here.
	h.forward(rw, req, delay)
}

// Start starts the execution of the proxy
func (p *proxy) Start() error {
	err := p.srv.Serve(p.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop stops the execution of the proxy
func (p *proxy) Stop() error {
	return p.srv.Shutdown(context.Background())
}

// Metrics returns runtime metrics for the proxy.
func (p *proxy) Metrics() map[string]uint {
	return p.metrics.Map()
}

// Force stops the proxy without waiting for connections to drain
func (p *proxy) Force() error {
	return p.srv.Close()
}

// supportedMetrics is a helper function that returns the metrics that the http proxy supports and thus should be
// pre-initialized to zero. This function is defined due to the testing limitations mentioned in
// https://github.com/grafana/xk6-disruptor/issues/314, as httpHandler tests currently need this information.
func supportedMetrics() []string {
	return []string{
		protocol.MetricRequests,
		protocol.MetricRequestsExcluded,
		protocol.MetricRequestsDisrupted,
	}
}
