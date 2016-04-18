package lua

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"github.com/yuin/gopher-lua"
	"time"
)

func (u *LuaVU) HTTPLoader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get": u.HTTPGet,
	})
	L.SetField(mod, "name", lua.LString("http"))
	L.Push(mod)
	return 1
}

func (u *LuaVU) HTTPGet(L *lua.LState) int {
	result := make(chan runner.Result, 1)
	go func() {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		req.SetRequestURI("http://google.com")

		startTime := time.Now()
		err := u.r.Client.Do(req, res)
		duration := time.Since(startTime)

		result <- runner.Result{Error: err, Time: duration}
	}()

	select {
	case <-u.ctx.Done():
	case res := <-result:
		u.ch <- res
	}

	return 0
}
