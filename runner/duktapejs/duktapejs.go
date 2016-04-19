package duktapejs

import (
	"errors"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
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

		c := duktape.New()

		c.PushGlobalObject()
		c.PushString("__id")
		c.PushInt(int(id))
		if !c.PutProp(-3) {
			ch <- runner.Result{Error: errors.New("Couldn't push __id")}
			return
		}

		if _, err := c.PushGlobalGoFunction("get", vu.HTTPGet); err != nil {
			ch <- runner.Result{Error: err}
			return
		}
		if _, err := c.PushGlobalGoFunction("sleep", vu.Sleep); err != nil {
			ch <- runner.Result{Error: err}
		}

		c.PushString(r.Source)
		c.PushString(r.Filename)
		if err := c.Pcompile(0); err != nil {
			ch <- runner.Result{Error: err}
			return
		}

		for {
			c.DupTop()
			if code := c.Pcall(0); code != duktape.ExecSuccess {
				e := c.SafeToString(-1)
				c.Pop()
				ch <- runner.Result{Error: errors.New(e)}
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
