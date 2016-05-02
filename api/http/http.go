package http

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"math"
	"time"
)

type context struct {
	client *fasthttp.Client
}

func New() map[string]interface{} {
	ctx := &context{
		client: &fasthttp.Client{
			Dial:                fasthttp.Dial,
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt64,
		},
	}
	return map[string]interface{}{
		"get": ctx.Get,
	}
}

func (ctx *context) Get(url string) <-chan runner.Result {
	ch := make(chan runner.Result, 1)
	go func() {
		defer close(ch)

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		req.SetRequestURI(url)

		startTime := time.Now()
		err := ctx.client.Do(req, res)
		duration := time.Since(startTime)

		ch <- runner.Result{Error: err, Time: duration}
	}()
	return ch
}
