package js

import (
	"context"
	"errors"
	"time"
	// log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"sync"
	"sync/atomic"
)

var ErrDefaultExport = errors.New("you must export a 'default' function")

const entrypoint = "__$$entrypoint$$__"

type Runner struct {
	Runtime      *Runtime
	DefaultGroup *lib.Group
	Groups       []*lib.Group
	Tests        []*lib.Test

	groupIDCounter int64
	groupsMutex    sync.Mutex
	testIDCounter  int64
	testsMutex     sync.Mutex
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

	r := &Runner{Runtime: runtime}
	r.DefaultGroup = lib.NewGroup("", nil, nil)
	r.Groups = []*lib.Group{r.DefaultGroup}

	return r, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	u := &VU{
		runner: r,
		vm:     r.Runtime.VM.Copy(),
		group:  r.DefaultGroup,
	}

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

func (r *Runner) GetGroups() []*lib.Group {
	return r.Groups
}

func (r *Runner) GetTests() []*lib.Test {
	return r.Tests
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

func (u *VU) Sleep(secs float64) {
	time.Sleep(time.Duration(secs * float64(time.Second)))
}

func (u *VU) DoGroup(call otto.FunctionCall) otto.Value {
	name := call.Argument(0).String()
	group, ok := u.group.Group(name, &(u.runner.groupIDCounter))
	if !ok {
		u.runner.groupsMutex.Lock()
		u.runner.Groups = append(u.runner.Groups, group)
		u.runner.groupsMutex.Unlock()
	}
	u.group = group
	defer func() { u.group = group.Parent }()

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
		for _, name := range obj.Keys() {
			val, err := obj.Get(name)
			if err != nil {
				panic(err)
			}

			result, err := Test(val, arg0)
			if err != nil {
				panic(err)
			}

			test, ok := u.group.Test(name, &(u.runner.testIDCounter))
			if !ok {
				u.runner.testsMutex.Lock()
				u.runner.Tests = append(u.runner.Tests, test)
				u.runner.testsMutex.Unlock()
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

func Test(val, arg0 otto.Value) (bool, error) {
	switch {
	case val.IsFunction():
		val, err := val.Call(otto.UndefinedValue(), arg0)
		if err != nil {
			return false, err
		}
		return Test(val, arg0)
	case val.IsBoolean():
		b, err := val.ToBoolean()
		if err != nil {
			return false, err
		}
		return b, nil
	case val.IsNumber():
		f, err := val.ToFloat()
		if err != nil {
			return false, err
		}
		return f != 0, nil
	case val.IsString():
		s, err := val.ToString()
		if err != nil {
			return false, err
		}
		return s != "", nil
	default:
		return false, nil
	}
}
