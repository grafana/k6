package js

import (
	"context"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
)

var ErrDefaultExport = errors.New("you must export a 'default' function")

const entrypoint = "__$$entrypoint$$__"

type Runner struct {
	Runtime *Runtime
}

func NewRunner(runtime *Runtime, exports otto.Value) (*Runner, error) {
	expObj := exports.Object()
	if expObj == nil {
		return nil, ErrDefaultExport
	}

	// Values "remember" which VM they belong to, so to get a callable that works across VM copies,
	// we have to stick it in the global scope, then retrieve it again from the new instance.
	callable, err := expObj.Get("default")
	if err != nil {
		return nil, err
	}
	if !callable.IsFunction() {
		return nil, ErrDefaultExport
	}
	if err := runtime.VM.Set(entrypoint, callable); err != nil {
		return nil, err
	}

	return &Runner{Runtime: runtime}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	u := &VU{runner: r, vm: r.Runtime.VM.Copy()}

	callable, err := u.vm.Get(entrypoint)
	if err != nil {
		return nil, err
	}
	u.callable = callable

	if err := u.vm.Set("__vu_impl__", u); err != nil {
		return nil, err
	}

	return u, nil
}

type VU struct {
	ID int64

	runner   *Runner
	vm       *otto.Otto
	callable otto.Value

	ctx   context.Context
	group *lib.Group
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	u.ctx = ctx
	if _, err := u.callable.Call(otto.UndefinedValue()); err != nil {
		u.ctx = nil
		return nil, err
	}
	u.ctx = nil
	return nil, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	return nil
}

func (u *VU) DoGroup(call otto.FunctionCall) otto.Value {
	name := call.Argument(0).String()
	fn := call.Argument(1)
	if !fn.IsFunction() {
		panic(call.Otto.MakeSyntaxError("fn must be a function"))
	}
	log.WithField("name", name).Info("Group")

	val, err := fn.Call(call.This)
	if err != nil {
		panic(err)
	}
	return val
}

func (u *VU) DoTest(call otto.FunctionCall) otto.Value {
	if len(call.ArgumentList) < 2 {
		return otto.UndefinedValue()
	}

	arg0 := call.Argument(0)
	for _, v := range call.ArgumentList[1:] {
		obj := v.Object()
		if obj == nil {
			panic(call.Otto.MakeTypeError("tests must be objects"))
		}
		for _, key := range obj.Keys() {
			val, err := obj.Get(key)
			if err != nil {
				panic(err)
			}

			var res bool

		typeSwitchLoop:
			for {
				switch {
				case val.IsFunction():
					val, err = val.Call(otto.UndefinedValue(), arg0)
					if err != nil {
						panic(err)
					}
					continue typeSwitchLoop
				case val.IsUndefined() || val.IsNull():
					res = false
				case val.IsBoolean():
					b, err := val.ToBoolean()
					if err != nil {
						panic(err)
					}
					res = b
				case val.IsNumber():
					f, err := val.ToFloat()
					if err != nil {
						panic(err)
					}
					res = (f != 0)
				case val.IsString():
					s, err := val.ToString()
					if err != nil {
						panic(err)
					}
					res = (s != "")
				}
				break
			}

			log.WithFields(log.Fields{
				"arg0": arg0,
				"key":  key,
				"res":  res,
			}).Info("Test")
		}
	}
	return otto.UndefinedValue()
}
