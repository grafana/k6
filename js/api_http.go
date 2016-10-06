package js

import (
	// "github.com/robertkrimen/otto"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

var (
	MetricReqs          = stats.New("http_reqs", stats.Counter)
	MetricReqDuration   = stats.New("http_req_duration", stats.Trend, stats.Time)
	MetricReqBlocked    = stats.New("http_req_blocked", stats.Trend, stats.Time)
	MetricReqLookingUp  = stats.New("http_req_looking_up", stats.Trend, stats.Time)
	MetricReqConnecting = stats.New("http_req_connecting", stats.Trend, stats.Time)
	MetricReqSending    = stats.New("http_req_sending", stats.Trend, stats.Time)
	MetricReqWaiting    = stats.New("http_req_waiting", stats.Trend, stats.Time)
	MetricReqReceiving  = stats.New("http_req_receiving", stats.Trend, stats.Time)
)

type HTTPResponse struct {
	Status int
}

func (a JSAPI) HTTPRequest(method, url, body string, params map[string]interface{}) map[string]interface{} {
	log.WithFields(log.Fields{
		"method": method,
		"url":    url,
		"body":   body,
	}).Debug("JS: Request")
	bodyReader := io.Reader(nil)
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		throw(a.vu.vm, err)
	}

	if h, ok := params["headers"]; ok {
		headers, ok := h.(map[string]interface{})
		if !ok {
			panic(a.vu.vm.MakeTypeError("headers must be an object"))
		}
		for key, v := range headers {
			value, ok := v.(string)
			if !ok {
				panic(a.vu.vm.MakeTypeError("header values must be strings"))
			}
			req.Header.Set(key, value)
		}
	}

	tracer := lib.Tracer{}
	res, err := a.vu.HTTPClient.Do(req.WithContext(httptrace.WithClientTrace(a.vu.ctx, tracer.Trace())))
	if err != nil {
		throw(a.vu.vm, err)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		throw(a.vu.vm, err)
	}
	res.Body.Close()

	trail := tracer.Done()
	t := time.Now()
	tags := map[string]string{
		"vu":     a.vu.IDString,
		"method": method,
		"url":    url,
	}
	a.vu.Samples = append(a.vu.Samples,
		stats.Sample{Metric: MetricReqs, Time: t, Tags: tags, Value: 1},
		stats.Sample{Metric: MetricReqDuration, Time: t, Tags: tags, Value: float64(trail.Duration)},
		stats.Sample{Metric: MetricReqBlocked, Time: t, Tags: tags, Value: float64(trail.Blocked)},
		stats.Sample{Metric: MetricReqLookingUp, Time: t, Tags: tags, Value: float64(trail.LookingUp)},
		stats.Sample{Metric: MetricReqConnecting, Time: t, Tags: tags, Value: float64(trail.Connecting)},
		stats.Sample{Metric: MetricReqSending, Time: t, Tags: tags, Value: float64(trail.Sending)},
		stats.Sample{Metric: MetricReqWaiting, Time: t, Tags: tags, Value: float64(trail.Waiting)},
		stats.Sample{Metric: MetricReqReceiving, Time: t, Tags: tags, Value: float64(trail.Receiving)},
	)

	return map[string]interface{}{
		"status": res.StatusCode,
		"body":   string(resBody),
	}
}

func (a JSAPI) HTTPSetMaxRedirects(n int) {
	a.vu.MaxRedirects = n
}
