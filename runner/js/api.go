package js

import (
	"errors"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"gopkg.in/olebedev/go-duktape.v2"
	"time"
)

type apiFunc func(r *Runner, c *duktape.Context, ch chan<- runner.Result) int

func apiHTTPDo(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	method := argString(c, 0)
	if method == "" {
		ch <- runner.Result{Error: errors.New("Missing method in http call")}
		return 0
	}

	url := argString(c, 1)
	if url == "" {
		ch <- runner.Result{Error: errors.New("Missing URL in http call")}
		return 0
	}

	args := struct {
		Report  bool              `json:"report"`
		Headers map[string]string `json:"headers"`
	}{}
	if err := argJSON(c, 2, &args); err != nil {
		ch <- runner.Result{Error: errors.New("Invalid arguments to http call")}
		return 0
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)

	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	startTime := time.Now()
	err := r.Client.Do(req, res)
	duration := time.Since(startTime)

	if args.Report {
		ch <- runner.Result{Error: err, Time: duration}
	}

	c.PushObject()
	{
		c.PushNumber(float64(res.StatusCode()))
		c.PutPropString(-2, "status")

		c.PushString(string(res.Body()))
		c.PutPropString(-2, "body")

		c.PushObject()
		res.Header.VisitAll(func(key, value []byte) {
			c.PushString(string(value))
			c.PutPropString(-2, string(key))
		})
		c.PutPropString(-2, "headers")
	}

	return 1
}

func apiHTTPSetMaxConnectionsPerHost(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	num := int(argNumber(c, 0))
	if num < 1 {
		ch <- runner.Result{Error: errors.New("Max connections per host must be at least 1")}
	}
	r.Client.MaxConnsPerHost = num
	return 0
}
