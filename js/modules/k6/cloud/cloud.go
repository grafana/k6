// Package cloud implements k6/cloud which lets script find out more about the Cloud execution.
package cloud

import (
	"errors"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the execution module.
	ModuleInstance struct {
		vu  modules.VU
		obj *sobek.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{vu: vu}
	rt := vu.Runtime()
	o := rt.NewObject()
	defProp := func(name string, getter func() (sobek.Value, error)) {
		err := o.DefineAccessorProperty(name, rt.ToValue(func() sobek.Value {
			obj, err := getter()
			if err != nil {
				common.Throw(rt, err)
			}
			return obj
		}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, err)
		}
	}
	defProp("testRunId", mi.testRunId)

	mi.obj = o

	return mi
}

// Exports returns the exports of the execution module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

var errRunInInitContext = errors.New("getting cloud information outside of the VU context is not supported")

// testRunId returns a sobek.Value(string) with the Cloud test run id.
func (mi *ModuleInstance) testRunId() (sobek.Value, error) {
	rt := mi.vu.Runtime()
	vuState := mi.vu.State()
	if vuState == nil {
		return sobek.Undefined(), errRunInInitContext
	}

	if !vuState.Options.TestRunID.Valid {
		return sobek.Undefined(), nil
	}

	return rt.ToValue(vuState.Options.TestRunID.String), nil
}
