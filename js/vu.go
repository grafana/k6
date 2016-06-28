package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"time"
)

var (
	mRequests = stats.Stat{Name: "requests", Type: stats.HistogramType, Intent: stats.TimeIntent}
	mErrors   = stats.Stat{Name: "errors", Type: stats.CounterType}
)

type HTTPParams struct {
	Quiet   bool
	Headers map[string]string
}

type HTTPResponse struct {
	Status  int
	Headers map[string]string
	Body    string
}

func (res HTTPResponse) ToValue(vm *otto.Otto) (otto.Value, error) {
	obj, err := Make(vm, "HTTPResponse")
	if err != nil {
		return otto.UndefinedValue(), err
	}

	obj.Set("status", res.Status)
	obj.Set("headers", res.Headers)
	obj.Set("body", res.Body)

	return vm.ToValue(obj)
}

func (u *VU) HTTPRequest(method, url, body string, params HTTPParams) (HTTPResponse, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod(method)

	if method == "GET" || method == "HEAD" {
		req.SetRequestURI(putBodyInURL(url, body))
	} else if body != "" {
		req.SetRequestURI(url)
		req.SetBodyString(body)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	startTime := time.Now()
	err := u.Client.Do(req, resp)
	duration := time.Since(startTime)

	tags := stats.Tags{
		"url":    url,
		"method": method,
		"status": resp.StatusCode(),
	}
	u.Collector.Add(stats.Point{
		Stat:   &mRequests,
		Tags:   tags,
		Values: stats.Values{"duration": float64(duration)},
	})

	if err != nil {
		u.Collector.Add(stats.Point{
			Stat:   &mErrors,
			Tags:   tags,
			Values: stats.Value(1),
		})
		return HTTPResponse{}, err
	}

	headers := make(map[string]string)
	resp.Header.VisitAll(func(key []byte, value []byte) {
		headers[string(key)] = string(value)
	})

	return HTTPResponse{
		Status:  resp.StatusCode(),
		Headers: headers,
		Body:    string(resp.Body()),
	}, nil
}

func (u *VU) Sleep(t float64) {
	time.Sleep(time.Duration(t * float64(time.Second)))
}

func (u *VU) Log(level, msg string, fields map[string]interface{}) {
	e := u.Runner.logger.WithFields(log.Fields(fields))

	switch level {
	case "debug":
		e.Debug(msg)
	case "info":
		e.Info(msg)
	case "warn":
		e.Warn(msg)
	case "error":
		e.Error(msg)
	}
}
