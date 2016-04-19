package ottojs

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"time"
)

func (vu *VUContext) HTTPGet(call otto.FunctionCall) otto.Value {
	result := make(chan runner.Result, 1)
	go func() {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		url := call.Argument(0).String()
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

	return otto.UndefinedValue()
}
