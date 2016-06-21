package js

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"os"
)

type Runner struct {
	Test speedboat.Test

	filename string
	source   string

	logger *log.Logger

	mDuration *sampler.Metric
	mErrors   *sampler.Metric
}

type VU struct {
	Runner *Runner
	VM     *otto.Otto
	Script *otto.Script

	Client fasthttp.Client

	ID        int64
	Iteration int64
}

func New(t speedboat.Test, filename, source string) *Runner {
	return &Runner{
		Test:     t,
		filename: filename,
		source:   source,
		logger: &log.Logger{
			Out:       os.Stderr,
			Formatter: &log.TextFormatter{},
			Hooks:     make(log.LevelHooks),
			Level:     log.DebugLevel,
		},
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

			body, err := bodyFromValue(call.Argument(2))
			if err != nil {
				panic(vm.MakeTypeError("invalid body"))
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
				panic(vm.MakeCustomError("Error", err.Error()))
			}

			return val
		},
		"setMaxConnsPerHost": func(call otto.FunctionCall) otto.Value {
			num, err := call.Argument(0).ToInteger()
			if err != nil {
				panic(vm.MakeTypeError("argument must be an integer"))
			}
			if num <= 0 {
				panic(vm.MakeRangeError("argument must be >= 1"))
			}
			if num > math.MaxInt32 {
				num = math.MaxInt32
			}

			vu.Client.MaxConnsPerHost = int(num)

			return otto.UndefinedValue()
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
		"id": func(call otto.FunctionCall) otto.Value {
			val, err := call.Otto.ToValue(vu.ID)
			if err != nil {
				panic(jsError(vm, err))
			}
			return val
		},
		"iteration": func(call otto.FunctionCall) otto.Value {
			val, err := call.Otto.ToValue(vu.Iteration)
			if err != nil {
				panic(jsError(vm, err))
			}
			return val
		},
	})
	vm.Set("$log", map[string]interface{}{
		"log": func(call otto.FunctionCall) otto.Value {
			level, err := call.Argument(0).ToString()
			if err != nil {
				panic(vm.MakeTypeError("level must be a string"))
			}

			msg, err := call.Argument(1).ToString()
			if err != nil {
				panic(vm.MakeTypeError("message must be a string"))
			}

			fields := make(map[string]interface{})
			fieldsObj := call.Argument(2).Object()
			if fieldsObj != nil {
				for _, key := range fieldsObj.Keys() {
					valObj, _ := fieldsObj.Get(key)
					val, err := valObj.Export()
					if err != nil {
						panic(err)
					}
					fields[key] = val
				}
			}

			vu.Log(level, msg, fields)

			return otto.UndefinedValue()
		},
	})

	init := `
	function HTTPResponse() {
		this.json = function() {
			return JSON.parse(this.body);
		};
	}
	
	$http.get = function(url, data, params) { return $http.request('GET', url, data, params); };
	$http.post = function(url, data, params) { return $http.request('POST', url, data, params); };
	$http.put = function(url, data, params) { return $http.request('PUT', url, data, params); };
	$http.delete = function(url, data, params) { return $http.request('DELETE', url, data, params); };
	$http.patch = function(url, data, params) { return $http.request('PATCH', url, data, params); };
	$http.options = function(url, data, params) { return $http.request('OPTIONS', url, data, params); };
	$http.head = function(url, data, params) { return $http.request('HEAD', url, data, params); };
	
	$log.debug = function(msg, fields) { $log.log('debug', msg, fields); };
	$log.info = function(msg, fields) { $log.log('info', msg, fields); };
	$log.warn = function(msg, fields) { $log.log('warn', msg, fields); };
	$log.error = function(msg, fields) { $log.log('error', msg, fields); };
	`
	if _, err := vm.Eval(init); err != nil {
		return nil, err
	}

	return &vu, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.Iteration = 0
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	u.Iteration++
	if _, err := u.VM.Run(u.Script); err != nil {
		return err
	}
	return nil
}
