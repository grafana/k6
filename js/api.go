package js

import (
	"github.com/robertkrimen/otto"
	"sync/atomic"
	"time"
)

type JSAPI struct {
	vu *VU
}

func (a JSAPI) Sleep(secs float64) {
	time.Sleep(time.Duration(secs * float64(time.Second)))
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
		panic(err)
	}
	return val
}

func (a JSAPI) DoTest(call otto.FunctionCall) otto.Value {
	if len(call.ArgumentList) < 2 {
		return otto.UndefinedValue()
	}

	arg0 := call.Argument(0)
	for _, v := range call.ArgumentList[1:] {
		obj := v.Object()
		if obj == nil {
			panic(call.Otto.MakeTypeError("tests must be objects"))
		}
		for _, name := range obj.Keys() {
			val, err := obj.Get(name)
			if err != nil {
				panic(err)
			}

			result, err := Test(val, arg0)
			if err != nil {
				panic(err)
			}

			test, ok := a.vu.group.Test(name, &(a.vu.runner.testIDCounter))
			if !ok {
				a.vu.runner.testsMutex.Lock()
				a.vu.runner.Tests = append(a.vu.runner.Tests, test)
				a.vu.runner.testsMutex.Unlock()
			}

			if result {
				atomic.AddInt64(&(test.Passes), 1)
			} else {
				atomic.AddInt64(&(test.Fails), 1)
			}
		}
	}
	return otto.UndefinedValue()
}
