package js

import (
	"errors"
	"github.com/GeertJohan/go.rice"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"strconv"
	"time"
)

type Runner struct {
	Client *fasthttp.Client

	lib    *rice.Box
	vendor *rice.Box
}

func New() *Runner {
	return &Runner{
		Client: &fasthttp.Client{
			MaxIdleConnDuration: time.Duration(0),
		},
		lib:    rice.MustFindBox("lib"),
		vendor: rice.MustFindBox("vendor"),
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

			c.PushObject()
			c.PutPropString(-2, "types")

			pushData(c, t, id)
			c.PutPropString(-2, "data")
		}
		c.PutPropString(-2, "__internal__")

		load := map[*rice.Box][]string{
			r.lib:    []string{"require.js", "http.js"},
			r.vendor: []string{"lodash/dist/lodash.min.js"},
		}
		for box, files := range load {
			for _, name := range files {
				src, err := box.String(name)
				if err != nil {
					ch <- runner.Result{Error: err}
					return
				}
				if err = loadFile(c, name, src); err != nil {
					ch <- runner.Result{Error: err}
					return
				}
			}
		}

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
			"do": apiHTTPDo,
			"setMaxConnectionsPerHost": apiHTTPSetMaxConnectionsPerHost,
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
		fn := fn
		c.PushGoFunction(func(lc *duktape.Context) int {
			return fn(r, lc, ch)
		})
		c.PutPropString(-2, name)
	}
}

func loadFile(c *duktape.Context, name, src string) error {
	c.PushString(name)
	if err := c.PcompileStringFilename(0, src); err != nil {
		return err
	}
	c.Pcall(0)
	c.Pop()
	return nil
}
