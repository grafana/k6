package js

import (
	"context"
	"errors"
	// log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"sync"
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

	if err := u.vm.Set("__jsapi__", JSAPI{u}); err != nil {
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
