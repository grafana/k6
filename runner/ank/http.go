package ank

import (
	"github.com/loadimpact/speedboat/runner"
	anko "github.com/mattn/anko/vm"
	"github.com/valyala/fasthttp"
	"time"
)

func (vu *VUContext) HTTPLoader(env *anko.Env) *anko.Env {
	m := env.NewPackage("http")
	m.Define("get", vu.HTTPGet)
	return m
}

func (vu *VUContext) HTTPGet(url string) {
	result := make(chan runner.Result, 1)
	go func() {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		req.SetRequestURI(url)

		startTime := time.Now()
		err := vu.r.Client.Do(req, res)
		duration := time.Since(startTime)

		result <- runner.Result{Error: err, Time: duration}
	}()

	select {
	case <-vu.ctx.Done():
	case res := <-result:
		vu.ch <- res
	}
}
