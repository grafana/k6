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

	httpClient *http.Client
}

func New() (r *JSRunner, err error) {
	r = &JSRunner{}

	// Create a base VM
	r.BaseVM = otto.New()

	// Bridge basic functions
	r.BaseVM.Set("sleep", jsSleepFactory(time.Sleep))

	// Use a single HTTP client for this
	r.httpClient = &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	r.BaseVM.Set("get", jsHTTPGetFactory(r.BaseVM, func(url string) (*http.Response, error) {
		return r.httpClient.Get(url)
	}))

	return r, nil
}

func (r *JSRunner) Load(filename, src string) (err error) {
	r.Script, err = r.BaseVM.Compile(filename, src)
	return err
}

func (r *JSRunner) RunVU(stop <-chan interface{}) <-chan interface{} {
	out := make(chan interface{})

	go func() {
		defer close(out)

	runLoop:
		for {
			select {
			case <-stop:
				break runLoop
			default:
				vm := r.BaseVM.Copy()
				for res := range r.RunIteration(vm) {
					out <- res
				}
			}
		}
	}()

	return out
}

func (r *JSRunner) RunIteration(vm *otto.Otto) <-chan interface{} {
	out := make(chan interface{})

	go func() {
		defer close(out)

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
