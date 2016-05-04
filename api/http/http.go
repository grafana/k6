package http

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"math"
	"time"
)

type context struct {
	client *fasthttp.Client
}

type RequestArgs struct {
	Follow bool `json:"follow"`
	Report bool `json:"report"`
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
		"get":     ctx.Get,
		"head":    ctx.Head,
		"post":    ctx.Post,
		"put":     ctx.Put,
		"delete":  ctx.Delete,
		"request": ctx.Request,
	}
}

func (ctx *context) Get(url string, args RequestArgs) <-chan runner.Result {
	return ctx.Request("GET", url, "", args)
}

func (ctx *context) Head(url string, args RequestArgs) <-chan runner.Result {
	return ctx.Request("HEAD", url, "", args)
}

func (ctx *context) Post(url, body string, args RequestArgs) <-chan runner.Result {
	return ctx.Request("POST", url, body, args)
}

func (ctx *context) Put(url, body string, args RequestArgs) <-chan runner.Result {
	return ctx.Request("PUT", url, body, args)
}

func (ctx *context) Delete(url, body string, args RequestArgs) <-chan runner.Result {
	return ctx.Request("DELETE", url, body, args)
}

func (ctx *context) Request(method, url, body string, args RequestArgs) <-chan runner.Result {
	log.WithFields(log.Fields{
		"method": method,
		"url":    url,
		"follow": args.Follow,
		"report": args.Report,
	}).Debug("Request")
	ch := make(chan runner.Result, 1)
	go func() {
		defer close(ch)

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		req.SetRequestURI(url)
		req.Header.SetMethod(method)
		req.SetBodyString(body)

		startTime := time.Now()
		err := ctx.client.Do(req, res)
		duration := time.Since(startTime)

		ch <- runner.Result{Error: err, Time: duration}
	}()
	return ch
}
