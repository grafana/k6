package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"time"
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
	return vm.ToValue(map[string]interface{}{
		"status":  res.Status,
		"headers": res.Headers,
		"body":    res.Body,
	})
}

func (u *VU) HTTPRequest(method, url, body string, params HTTPParams) (HTTPResponse, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	if body != "" {
		req.SetBodyString(body)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	startTime := time.Now()
	err := u.Client.Do(req, resp)
	duration := time.Since(startTime)

	u.Runner.mDuration.WithFields(sampler.Fields{
		"url":    u.Runner.Test.URL,
		"method": "GET",
		"status": resp.StatusCode(),
	}).Duration(duration)

	if err != nil {
		u.Runner.mErrors.WithField("url", u.Runner.Test.URL).Int(1)
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
