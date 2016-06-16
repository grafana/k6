package js

import (
	"fmt"
	"github.com/loadimpact/speedboat"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"os"
)

type Runner struct {
	Test speedboat.Test

	filename string
	source   string
}

type VU struct {
	Runner *Runner
	VM     *otto.Otto
	Script *otto.Script

	Client fasthttp.Client

	ID int64
}

func New(t speedboat.Test, filename, source string) *Runner {
	return &Runner{
		Test:     t,
		filename: filename,
		source:   source,
	}
}

func (r *Runner) NewVU() (speedboat.VU, error) {
	vm := otto.New()

	script, err := vm.Compile(r.filename, r.source)
	if err != nil {
		return nil, err
	}

	vm.Set("print", func(call otto.FunctionCall) otto.Value {
		fmt.Fprintln(os.Stderr, call.Argument(0))
		return otto.UndefinedValue()
	})

	return &VU{
		Runner: r,
		VM:     vm,
		Script: script,
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	if _, err := u.VM.Run(u.Script); err != nil {
		return err
	}
	return nil
}
