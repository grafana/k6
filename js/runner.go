package js

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"os"
)

type Runner struct {
	Test speedboat.Test

	filename string
	source   string

	mDuration *sampler.Metric
	mErrors   *sampler.Metric
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
		Test:      t,
		filename:  filename,
		source:    source,
		mDuration: sampler.Stats("request.duration"),
		mErrors:   sampler.Counter("request.error"),
	}
}

func (r *Runner) NewVU() (speedboat.VU, error) {
	vm := otto.New()

	script, err := vm.Compile(r.filename, r.source)
	if err != nil {
		return nil, err
	}

	vu := VU{
		Runner: r,
		VM:     vm,
		Script: script,
	}

	vm.Set("print", func(call otto.FunctionCall) otto.Value {
		fmt.Fprintln(os.Stderr, call.Argument(0))
		return otto.UndefinedValue()
	})

	vm.Set("$http", map[string]interface{}{
		"request": func(call otto.FunctionCall) otto.Value {
			method, err := call.Argument(0).ToString()
			if err != nil {
				panic(vm.MakeTypeError("method must be a string"))
			}

			url, err := call.Argument(1).ToString()
			if err != nil {
				panic(vm.MakeTypeError("url must be a string"))
			}

			var body string
			bodyArg := call.Argument(2)
			if !bodyArg.IsUndefined() && !bodyArg.IsNull() {
				body, err = bodyArg.ToString()
				if err != nil {
					panic(vm.MakeTypeError("body must be a string"))
				}
			}

			params, err := paramsFromObject(call.Argument(3).Object())
			if err != nil {
				panic(err)
			}

			log.WithFields(log.Fields{
				"method": method,
				"url":    url,
				"body":   body,
				"params": params,
			}).Debug("Request")
			res, err := vu.HTTPRequest(method, url, body, params)
			if err != nil {
				panic(vm.MakeCustomError("HTTPError", err.Error()))
			}

			val, err := res.ToValue(vm)
			if err != nil {
				panic(err)
			}

			return val
		},
	})
	vm.Set("$vu", map[string]interface{}{
		"sleep": func(call otto.FunctionCall) otto.Value {
			t, err := call.Argument(0).ToFloat()
			if err != nil {
				panic(vm.MakeTypeError("time must be a number"))
			}

			vu.Sleep(t)

			return otto.UndefinedValue()
		},
	})

	init := `
	$http.get = function(url, data, params) { return $http.request('GET', url, data, params); };
	$http.post = function(url, data, params) { return $http.request('POST', url, data, params); };
	$http.put = function(url, data, params) { return $http.request('PUT', url, data, params); };
	$http.delete = function(url, data, params) { return $http.request('DELETE', url, data, params); };
	$http.patch = function(url, data, params) { return $http.request('PATCH', url, data, params); };
	$http.options = function(url, data, params) { return $http.request('OPTIONS', url, data, params); };
	$http.head = function(url, data, params) { return $http.request('HEAD', url, data, params); };
	`
	if _, err := vm.Eval(init); err != nil {
		return nil, err
	}

	return &vu, nil
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
