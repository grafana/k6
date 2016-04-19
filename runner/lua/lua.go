package lua

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"github.com/yuin/gopher-lua"
	"golang.org/x/net/context"
	"math"
	"time"
)

type LuaRunner struct {
	Filename string
	Source   string
	Client   *fasthttp.Client
}

type VUContext struct {
	r   *LuaRunner
	ctx context.Context
	ch  chan runner.Result
}

func New(filename, src string) *LuaRunner {
	return &LuaRunner{
		Filename: filename,
		Source:   src,
		Client: &fasthttp.Client{
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt32,
		},
	}
}

func (r *LuaRunner) Run(ctx context.Context) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		vu := VUContext{r: r, ctx: ctx, ch: ch}

		L := lua.NewState()
		defer L.Close()

		L.SetGlobal("sleep", L.NewFunction(vu.Sleep))

		L.PreloadModule("http", vu.HTTPLoader)

		// Try to load the script, abort execution if it fails
		lfn, err := L.LoadString(r.Source)
		if err != nil {
			ch <- runner.Result{Error: err}
			return
		}

		for {
			L.Push(lfn)
			if err := L.PCall(0, 0, nil); err != nil {
				ch <- runner.Result{Error: err}
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch
}
