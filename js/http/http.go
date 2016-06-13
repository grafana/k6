package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	neturl "net/url"
	"time"
)

type ContextKey int

const (
	clientKey = ContextKey(iota)
)

var ErrNoClient = errors.New("No client in context")

var mDuration *sampler.Metric
var mErrors *sampler.Metric

func init() {
	mDuration = sampler.Stats("request.duration")
	mErrors = sampler.Counter("request.error")
}

type Args struct {
	Quiet   bool              `json:"quiet"`
	Headers map[string]string `json:"headers"`
}

type Response struct {
	Status  int               `json:"status"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

func WithDefaultClient(ctx context.Context) context.Context {
	return WithClient(ctx, &fasthttp.Client{})
}

func WithClient(ctx context.Context, c *fasthttp.Client) context.Context {
	return context.WithValue(ctx, clientKey, c)
}

func GetClient(ctx context.Context) *fasthttp.Client {
	return ctx.Value(clientKey).(*fasthttp.Client)
}

func Do(ctx context.Context, method, url, body string, args Args) (Response, error) {
	client := GetClient(ctx)
	if client == nil {
		return Response{}, ErrNoClient
	}

	if method == "GET" && body != "" {
		u, err := neturl.Parse(url)
		if err != nil {
			return Response{}, err
		}

		var params map[string]interface{}
		if err = json.Unmarshal([]byte(body), &params); err != nil {
			return Response{}, err
		}

		q := u.Query()
		for key, val := range params {
			q.Set(key, fmt.Sprint(val))
		}
		u.RawQuery = q.Encode()
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
	err := client.Do(req, res)
	duration := time.Since(startTime)

	if !args.Quiet {
		mDuration.WithFields(sampler.Fields{
			"url":    url,
			"method": method,
			"status": res.StatusCode(),
		}).Duration(duration)
	}

	if err != nil {
		if !args.Quiet {
			mErrors.WithFields(sampler.Fields{
				"url":    url,
				"method": method,
				"error":  err,
			}).Int(1)
		}
		return Response{}, err
	}

	resHeaders := make(map[string]string)
	res.Header.VisitAll(func(key, value []byte) {
		resHeaders[string(key)] = string(value)
	})

	return Response{
		Status:  res.StatusCode(),
		Body:    string(res.Body()),
		Headers: resHeaders,
	}, nil
}
