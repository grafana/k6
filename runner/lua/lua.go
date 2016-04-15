package lua

import (
	"github.com/loadimpact/speedboat/runner"
	"golang.org/x/net/context"
	// "github.com/Shopify/go-lua"
)

type LuaRunner struct {
	Script string
}

func New(filename, src string) *LuaRunner {
	return &LuaRunner{}
}

func (r *LuaRunner) Run(ctx context.Context) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)
	}()

	return ch
}
