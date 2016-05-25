package js

import (
	"errors"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"strconv"
	"time"
)

const srcRequire = `
function require(name) {
	var mod = __internal__.modules[name];
	if (!mod) {
		throw new Error("Unknown module: " + name);
	}
	return mod;
}
`

type Runner struct {
	Client *fasthttp.Client
}

func New() *Runner {
	return &Runner{
		Client: &fasthttp.Client{
			MaxIdleConnDuration: time.Duration(0),
		},
	}
}

func (r *Runner) Run(ctx context.Context, t loadtest.LoadTest, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		c := duktape.New()
		defer c.Destroy()

		c.PushGlobalObject()

		c.PushObject()
		{
			pushModules(c, r, ch)
			c.PutPropString(-2, "modules")

			pushData(c, t, id)
			c.PutPropString(-2, "data")
		}
		c.PutPropString(-2, "__internal__")

		if err := c.PcompileString(duktape.CompileFunction|duktape.CompileStrict, srcRequire); err != nil {
			ch <- runner.Result{Error: err}
			return
		}
		c.PutPropString(-2, "require")

		// This will probably be moved to a module; global for compatibility
		c.PushGoFunction(func(c *duktape.Context) int {
			t := argNumber(c, 0)
			time.Sleep(time.Duration(t) * time.Second)
			return 0
		})
		c.PutPropString(-2, "sleep")

		if top := c.GetTopIndex(); top != 0 {
			panic("PROGRAMMING ERROR: Excess items on stack: " + strconv.Itoa(top+1))
		}

		// It should be cheaper memory-wise to keep the code on the global object (where it's
		// safe from GC shenanigans) and duplicate the key every iteration, than to keep it on
		// the stack and duplicate the whole function every iteration; just make it ABUNDANTLY
		// CLEAR that the script should NEVER touch that property or dumb things will happen
		codeProp := "__!!__seriously__don't__touch__from__script__!!__"
		c.PushString("script")
		if err := c.PcompileStringFilename(0, t.Source); err != nil {
			ch <- runner.Result{Error: err}
			return
		}
		c.PutPropString(-2, codeProp)

		c.PushString(codeProp)
		for {
			select {
			default:
				c.DupTop()
				if c.PcallProp(-3, 0) != duktape.ErrNone {
					c.GetPropString(-1, "fileName")
					filename := c.SafeToString(-1)
					c.Pop()

					c.GetPropString(-1, "lineNumber")
					line := c.ToNumber(-1)
					c.Pop()

					err := errors.New(c.SafeToString(-1))
					ch <- runner.Result{Error: err, Extra: map[string]interface{}{
						"file": filename,
						"line": line,
					}}
				}
				c.Pop() // Pop return value
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

func pushData(c *duktape.Context, t loadtest.LoadTest, id int64) {
	c.PushObject()

	c.PushInt(int(id))
	c.PutPropString(-2, "id")

	c.PushObject()
	{
		c.PushString(t.URL)
		c.PutPropString(-2, "url")
	}
	c.PutPropString(-2, "test")
}

func pushModules(c *duktape.Context, r *Runner, ch chan<- runner.Result) {
	c.PushObject()

	api := map[string]map[string]apiFunc{
		"http": map[string]apiFunc{
			"get": apiHTTPGet,
		},
	}
	for name, mod := range api {
		pushModule(c, r, ch, mod)
		c.PutPropString(-2, name)
	}
}

func pushModule(c *duktape.Context, r *Runner, ch chan<- runner.Result, members map[string]apiFunc) {
	c.PushObject()

	for name, fn := range members {
		c.PushGoFunction(func(lc *duktape.Context) int {
			return fn(r, lc, ch)
		})
		c.PutPropString(-2, name)
	}
}
