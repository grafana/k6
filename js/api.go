package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/robertkrimen/otto"
	"strconv"
	"sync/atomic"
	"time"
)

type JSAPI struct {
	vu *VU
}

func (a JSAPI) Sleep(secs float64) {
	time.Sleep(time.Duration(secs * float64(time.Second)))
}

func (a JSAPI) Log(level int, msg string, args []otto.Value) {
	fields := make(log.Fields, len(args))
	for i, arg := range args {
		if arg.IsObject() {
			obj := arg.Object()
			for _, key := range obj.Keys() {
				v, err := obj.Get(key)
				if err != nil {
					throw(a.vu.vm, err)
				}
				fields[key] = v.String()
			}
			continue
		}
		fields["arg"+strconv.FormatInt(int64(i), 10)] = arg.String()
	}

	entry := log.WithFields(fields)
	switch level {
	case 0:
		entry.Debug(msg)
	case 1:
		entry.Info(msg)
	case 2:
		entry.Warn(msg)
	case 3:
		entry.Error(msg)
	}
}

func (a JSAPI) DoGroup(call otto.FunctionCall) otto.Value {
	name := call.Argument(0).String()
	group, ok := a.vu.group.Group(name, &(a.vu.runner.groupIDCounter))
	if !ok {
		a.vu.runner.groupsMutex.Lock()
		a.vu.runner.Groups = append(a.vu.runner.Groups, group)
		a.vu.runner.groupsMutex.Unlock()
	}
	a.vu.group = group
	defer func() { a.vu.group = group.Parent }()

	fn := call.Argument(1)
	if !fn.IsFunction() {
		panic(call.Otto.MakeSyntaxError("fn must be a function"))
	}

	val, err := fn.Call(call.This)
	if err != nil {
		throw(call.Otto, err)
	}

	if val.IsUndefined() {
		return otto.TrueValue()
	}
	return val
}

func (a JSAPI) DoCheck(call otto.FunctionCall) otto.Value {
	if len(call.ArgumentList) < 2 {
		return otto.UndefinedValue()
	}

	success := true
	arg0 := call.Argument(0)
	for _, v := range call.ArgumentList[1:] {
		obj := v.Object()
		if obj == nil {
			panic(call.Otto.MakeTypeError("checks must be objects"))
		}
		for _, name := range obj.Keys() {
			val, err := obj.Get(name)
			if err != nil {
				throw(call.Otto, err)
			}

			result, err := Check(val, arg0)
			if err != nil {
				throw(call.Otto, err)
			}

			check, ok := a.vu.group.Check(name, &(a.vu.runner.checkIDCounter))
			if !ok {
				a.vu.runner.checksMutex.Lock()
				a.vu.runner.Checks = append(a.vu.runner.Checks, check)
				a.vu.runner.checksMutex.Unlock()
			}

			if result {
				atomic.AddInt64(&(check.Passes), 1)
			} else {
				atomic.AddInt64(&(check.Fails), 1)
				success = false
			}
		}
	}

	if !success {
		a.vu.Taint = true
		return otto.FalseValue()
	}
	return otto.TrueValue()
}

func (a JSAPI) Taint() {
	a.vu.Taint = true
}

func (a JSAPI) ElapsedMs() float64 {
	return float64(time.Since(a.vu.started)) * float64(time.Millisecond)
}
