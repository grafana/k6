package js

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/robertkrimen/otto"
	"time"
)

type JSRunner struct {
	BaseVM *otto.Otto
}

func New() (r *JSRunner, err error) {
	r = &JSRunner{}

	// Create a base VM
	r.BaseVM = otto.New()

	// TODO: Bridge functions here

	return r, nil
}

func (r *JSRunner) Run(filename, src string) <-chan runner.Result {
	out := make(chan runner.Result)

	go func() {
		out <- runner.Result{
			Type: "log",
			LogEntry: runner.LogEntry{
				Time: time.Now(),
				Text: "AaaaaaA",
			},
		}
		close(out)
	}()

	return out
}
