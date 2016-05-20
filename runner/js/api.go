package js

import (
	"github.com/loadimpact/speedboat/runner"
	"gopkg.in/olebedev/go-duktape.v2"
)

type apiFunc func(c *duktape.Context, ch <-chan runner.Result) int

func apiHTTPGet(c *duktape.Context, ch <-chan runner.Result) int {
	return 0
}
