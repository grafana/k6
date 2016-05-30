package js

import (
	"encoding/json"
	"fmt"
	"github.com/valyala/fasthttp"
	neturl "net/url"
	"time"
)

type httpArgs struct {
	Quiet   bool              `json:"quiet"`
	Headers map[string]string `json:"headers"`
}

type httpResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

func httpDo(c *fasthttp.Client, method, url, body string, args httpArgs) (httpResponse, time.Duration, error) {
	if method == "GET" && body != "" {
		u, err := neturl.Parse(url)
		if err != nil {
			return httpResponse{}, 0, err
		}

		var params map[string]interface{}
		if err = json.Unmarshal([]byte(body), &params); err != nil {
			return httpResponse{}, 0, err
		}

		for key, val := range params {
			u.Query().Set(key, fmt.Sprint(val))
		}
		url = u.String()
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)

	if method != "GET" {
		req.SetBodyString(body)
	}

	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	startTime := time.Now()
	err := c.Do(req, res)
	duration := time.Since(startTime)

	if err != nil {
		return httpResponse{}, duration, err
	}

	resHeaders := make(map[string]string)
	res.Header.VisitAll(func(key, value []byte) {
		resHeaders[string(key)] = string(value)
	})

	return httpResponse{
		Status:  res.StatusCode(),
		Body:    string(res.Body()),
		Headers: resHeaders,
	}, duration, nil
}
