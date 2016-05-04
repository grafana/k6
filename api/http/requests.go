package http

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"time"
)

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
	args.ApplyDefaults(ctx.defaults)
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
		req.Header.SetUserAgent(args.UserAgent)
		req.SetBodyString(body)

		startTime := time.Now()
		err := ctx.client.Do(req, res)
		duration := time.Since(startTime)

		ch <- runner.Result{Error: err, Time: duration}
	}()
	return ch
}
