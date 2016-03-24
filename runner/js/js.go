package js

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/robertkrimen/otto"
	"net/http"
	"time"
)

type JSRunner struct {
	BaseVM *otto.Otto
	Script *otto.Script
}

func New() (r *JSRunner, err error) {
	r = &JSRunner{}

	// Create a base VM
	r.BaseVM = otto.New()

	// Bridge basic functions
	r.BaseVM.Set("sleep", jsSleepFactory(time.Sleep))
	r.BaseVM.Set("get", jsHTTPGetFactory(r.BaseVM, http.Get))

	return r, nil
}

func (r *JSRunner) Load(filename, src string) (err error) {
	r.Script, err = r.BaseVM.Compile(filename, src)
	return err
}

func (r *JSRunner) RunVU() <-chan interface{} {
	out := make(chan interface{})

	go func() {
		defer close(out)

		vm := r.BaseVM.Copy()
		for res := range r.RunIteration(vm) {
			out <- res
		}
	}()

	return out
}

func (r *JSRunner) RunIteration(vm *otto.Otto) <-chan interface{} {
	out := make(chan interface{})

	go func() {
		defer close(out)
		defer func() {
			if err := recover(); err != nil {
				out <- runner.NewError(err.(error))
			}
		}()

		// Log has to be bridged here, as it needs a reference to the channel
		vm.Set("log", jsLogFactory(func(text string) {
			out <- runner.NewLogEntry(text)
		}))

		startTime := time.Now()
		_, err := vm.Run(r.Script)
		duration := time.Since(startTime)

		if err != nil {
			out <- runner.NewError(err)
		}

		out <- runner.NewMetric(startTime, duration)
	}()

	return out
}
