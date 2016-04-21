package ank

import (
	"errors"
	"fmt"
	"github.com/loadimpact/speedboat/runner"
	anko_core "github.com/mattn/anko/builtins"
	anko "github.com/mattn/anko/vm"
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

		vm := anko.NewEnv()
		anko_core.Import(vm)

		vm.Set("__id", id)
		vm.Define("sleep", vu.Sleep)

		pkgs := map[string]func(env *anko.Env) *anko.Env{
			"http": vu.HTTPLoader,
		}
		vm.Define("import", func(s string) interface{} {
			if loader, ok := pkgs[s]; ok {
				m := loader(vm)
				return m
			}
			ch <- runner.Result{Error: errors.New(fmt.Sprintf("Package not found: %s", s))}
			return nil
		})

		for {
			if _, err := vm.Execute(r.Source); err != nil {
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
