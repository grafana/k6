package js

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"time"
)

var (
	mRequests = stats.Stat{Name: "requests", Type: stats.HistogramType, Intent: stats.TimeIntent}
	mErrors   = stats.Stat{Name: "errors", Type: stats.CounterType}

	ErrTooManyRedirects = errors.New("too many redirects")
)

type HTTPParams struct {
	Follow  bool
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

func (u *VU) HTTPRequest(method, url, body string, params HTTPParams, redirects int) (HTTPResponse, error) {
	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return HTTPResponse{}, err
	}

	req := http.Request{
		Method: method,
		URL:    parsedURL,
		Header: make(http.Header),
	}

	if method == "GET" || method == "HEAD" {
		req.URL.RawQuery = body
	} else {
		// NOT IMPLEMENTED! I'm just testing stuff out.
		// req.SetBodyString(body)
	}

	for key, value := range params.Headers {
		req.Header[key] = []string{value}
	}

	startTime := time.Now()
	resp, err := u.Client.Do(&req)
	duration := time.Since(startTime)

	var status int
	var respBody []byte
	if err == nil {
		status = resp.StatusCode
		respBody, _ = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}

	tags := stats.Tags{
		"url":    url,
		"method": method,
		"status": status,
	}

	if !params.Quiet {
		u.Collector.Add(stats.Sample{
			Stat:   &mRequests,
			Tags:   tags,
			Values: stats.Values{"duration": float64(duration)},
		})
	}

	if err != nil {
		if !params.Quiet {
			u.Collector.Add(stats.Sample{
				Stat:   &mErrors,
				Tags:   tags,
				Values: stats.Value(1),
			})
		}
		return HTTPResponse{}, err
	}

	// switch resp.StatusCode {
	// case 301, 302, 303, 307, 308:
	// 	if !params.Follow {
	// 		break
	// 	}
	// 	if redirects >= u.FollowDepth {
	// 		return HTTPResponse{}, ErrTooManyRedirects
	// 	}

	// 	redirectURL := url
	// 	resp.Header.VisitAll(func(key, value []byte) {
	// 		if string(key) != "Location" {
	// 			return
	// 		}

	// 		redirectURL = resolveRedirect(url, string(value))
	// 	})

	// 	redirectMethod := method
	// 	redirectBody := body
	// 	if status == 301 || status == 302 || status == 303 {
	// 		redirectMethod = "GET"
	// 		redirectBody = ""
	// 	}
	// 	return u.HTTPRequest(redirectMethod, redirectURL, redirectBody, params, redirects+1)
	// }

	headers := make(map[string]string)
	for key, vals := range resp.Header {
		headers[key] = vals[0]
	}

	return HTTPResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(respBody),
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
