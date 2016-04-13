package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/util"
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
	r.BaseVM.Set("log", jsLogFactory(func(text string) {
		// out <- runner.NewLogEntry(text)
		log.WithField("text", text).Info("Test Log")
	}))

	// Use a single HTTP client for this
	r.httpClient = &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	r.BaseVM.Set("get", jsHTTPGetFactory(func(url string) (*http.Response, error) {
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
				for res := range r.RunIteration() {
					out <- res
				}
			}
		}
	}()

	return out
}

func (r *JSRunner) RunIteration() <-chan interface{} {
	out := make(chan interface{})

	go func() {
		defer close(out)
		defer func() {
			if err := recover(); err != nil {
				out <- runner.NewError(err.(JSError))
			}
		}()

		// Make a copy of the base VM
		vm := r.BaseVM //.Copy()

		// Log has to be bridged here, as it needs a reference to the channel
		// vm.Set("log", jsLogFactory(func(text string) {
		// 	out <- runner.NewLogEntry(text)
		// }))

		startTime := time.Now()
		var err error
		duration := util.Time(func() {
			_, err = vm.Run(r.Script)
		})

		if err != nil {
			out <- runner.NewError(err)
		}

		out <- runner.NewMetric(startTime, duration)
	}()

	return out
}
