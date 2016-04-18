package lua

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"github.com/yuin/gopher-lua"
	"time"
)

func (vu *VUContext) HTTPLoader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get": vu.HTTPGet,
	})
	L.SetField(mod, "name", lua.LString("http"))
	L.Push(mod)
	return 1
}

func (vu *VUContext) HTTPGet(L *lua.LState) int {
	result := make(chan runner.Result, 1)
	go func() {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		req.SetRequestURI("http://google.com")

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

	return 0
}
