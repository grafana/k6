package js

import (
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
		setupInternals(c, t, id, ch)
	}()

	return ch
}

func setupInternals(c *duktape.Context, t loadtest.LoadTest, id int64, ch <-chan runner.Result) {
	api := map[string]map[string]apiFunc{
		"http": map[string]apiFunc{
			"get": apiHTTPGet,
		},
	}

	c.PushGlobalObject()

	c.PushObject() // __internal__

	c.PushObject() // __internal__.modules
	for name, mod := range api {
		pushModule(c, ch, mod)
		c.PutPropString(-2, name)
	}
	c.PutPropString(-2, "modules")

	c.PutPropString(-2, "__internal__")

	c.Pop() // global object

	if top := c.GetTopIndex(); !(top < 0) { // < 0 = invalid index = empty stack
		panic("PROGRAMMING ERROR: Stack depth must be 0, is: " + strconv.Itoa(top+1))
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
