package js

import (
	"errors"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"strconv"
)

type Runner struct {
}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context, t loadtest.LoadTest, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		c := duktape.New()
		defer c.Destroy()

		c.PushGlobalObject()
		c.PushObject() // __internal__

		pushModules(c, t, id, ch)
		c.PutPropString(-2, "modules")

		c.PutPropString(-2, "__internal__")

		if top := c.GetTopIndex(); top != 0 {
			panic("PROGRAMMING ERROR: Excess items on stack: " + strconv.Itoa(top+1))
		}

		if err := c.PcompileString(0, t.Source); err != nil {
			ch <- runner.Result{Error: err}
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				c.DupTop()
				if c.Pcall(0) != duktape.ErrNone {
					err := errors.New(c.SafeToString(-1))
					ch <- runner.Result{Error: err}
				}
				c.Pop() // Pop return value
			}
		}
	}()

	return ch
}

func pushModules(c *duktape.Context, t loadtest.LoadTest, id int64, ch <-chan runner.Result) {
	api := map[string]map[string]apiFunc{
		"http": map[string]apiFunc{
			"get": apiHTTPGet,
		},
	}

	c.PushObject() // __internal__.modules
	for name, mod := range api {
		pushModule(c, ch, mod)
		c.PutPropString(-2, name)
	}
}

func pushModule(c *duktape.Context, ch <-chan runner.Result, members map[string]apiFunc) int {
	idx := c.PushObject()

	for name, fn := range members {
		c.PushGoFunction(func(lc *duktape.Context) int {
			return fn(lc, ch)
		})
		c.PutPropString(idx, name)
	}

	return idx
}
