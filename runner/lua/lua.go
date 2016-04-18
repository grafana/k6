package lua

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/yuin/gopher-lua"
	"golang.org/x/net/context"
)

type LuaRunner struct {
	Filename, Source string
}

func New(filename, src string) *LuaRunner {
	return &LuaRunner{
		Filename: filename,
		Source:   src,
	}
}

func (r *LuaRunner) Run(ctx context.Context) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		L := lua.NewState()
		defer L.Close()

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
