package js

import (
	// "github.com/robertkrimen/otto"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"strconv"
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
		"vu":       a.vu.IDString,
		"method":   method,
		"url":      url,
		"group_id": strconv.FormatInt(a.vu.group.ID, 10),
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

	headers := make(map[string]string)
	for k, v := range res.Header {
		headers[k] = strings.Join(v, ", ")
	}
	return map[string]interface{}{
		"status":  res.StatusCode,
		"body":    string(resBody),
		"headers": headers,
		"timings": map[string]float64{
			"duration":   float64(trail.Duration) / float64(time.Millisecond),
			"blocked":    float64(trail.Blocked) / float64(time.Millisecond),
			"looking_up": float64(trail.LookingUp) / float64(time.Millisecond),
			"connecting": float64(trail.Connecting) / float64(time.Millisecond),
			"sending":    float64(trail.Sending) / float64(time.Millisecond),
			"waiting":    float64(trail.Waiting) / float64(time.Millisecond),
			"receiving":  float64(trail.Receiving) / float64(time.Millisecond),
		},
	}
}

func (a JSAPI) HTTPSetMaxRedirects(n int) {
	a.vu.MaxRedirects = n
}
