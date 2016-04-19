package ottojs

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"time"
)

type Runner struct {
	Filename string
	Source   string
	Client   *fasthttp.Client
}

type VUContext struct {
	r   *Runner
	ctx context.Context
	ch  chan runner.Result
}

func New(filename, src string) *Runner {
	return &Runner{
		Filename: filename,
		Source:   src,
		Client: &fasthttp.Client{
			Dial:                fasthttp.Dial,
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt64,
		},
	}
}
func (r *Runner) Run(ctx context.Context, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		vu := VUContext{r: r, ctx: ctx, ch: ch}

		vm := otto.New()
		vm.Set("__id", id)
		vm.Set("get", vu.HTTPGet)
		vm.Set("sleep", vu.Sleep)

		script, err := vm.Compile(r.Filename, r.Source)
		if err != nil {
			ch <- runner.Result{Error: err}
			return
		}

		for {
			if _, err := vm.Run(script); err != nil {
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
