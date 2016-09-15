package js

import (
	"context"
	"errors"
	// log "github.com/Sirupsen/logrus"
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
	vm := r.Runtime.VM.Copy()
	callable, err := vm.Get(entrypoint)
	if err != nil {
		return nil, err
	}

	return &VU{runner: r, vm: vm, callable: callable}, nil
}

type VU struct {
	ID int64

	runner   *Runner
	vm       *otto.Otto
	callable otto.Value

	ctx context.Context
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
