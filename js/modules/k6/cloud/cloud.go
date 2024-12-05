// Package cloud implements k6/cloud which lets script find out more about the Cloud execution.
package cloud

import (
	"sync"

	"github.com/grafana/sobek"
	"github.com/mstoykov/envconfig"

	"go.k6.io/k6/cloudapi"
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

		once      sync.Once
		testRunID sobek.Value
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

	mi.obj = rt.NewObject()
	defProp := func(name string, getter func() (sobek.Value, error)) {
		err := mi.obj.DefineAccessorProperty(name, rt.ToValue(func() sobek.Value {
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

	// By default, we try to load the test run id from the environment variables,
	// which corresponds to those scenarios where the k6 binary is running in the Cloud.
	var envConf cloudapi.Config
	if err := envconfig.Process("", &envConf, vu.InitEnv().LookupEnv); err != nil {
		common.Throw(vu.Runtime(), err)
	}
	if envConf.TestRunID.Valid {
		mi.testRunID = mi.vu.Runtime().ToValue(envConf.TestRunID.String)
	} else {
		mi.testRunID = sobek.Undefined() // Default value.
	}

	return mi
}

// Exports returns the exports of the execution module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

// testRunId returns a sobek.Value(string) with the Cloud test run id.
//
// This code can be executed in two situations, either when the k6 binary is running in the Cloud, in which case
// the value of the test run id would be available in the environment, and we would have loaded at module initialization
// time; or when the k6 binary is running locally and test run id is present in the options, which we try to read at
// time of running this method, but only once for the whole execution as options won't change anymore.
func (mi *ModuleInstance) testRunId() (sobek.Value, error) {
	// In case we have a value (e.g. loaded from env), we return it.
	// If we're in the init context (where we cannot read the options), we return undefined (the default value).
	if !sobek.IsUndefined(mi.testRunID) || mi.vu.State() == nil {
		return mi.testRunID, nil
	}

	// Otherwise, we try to read the test run id from options.
	// We only try it once for the whole execution, as options won't change.
	vuState := mi.vu.State()
	var err error
	mi.once.Do(func() {
		// We pass almost all values to zero/nil because here we only care about the Cloud configuration present in options.
		var optsConf cloudapi.Config
		optsConf, _, err = cloudapi.GetConsolidatedConfig(vuState.Options.Cloud, nil, "", nil, nil)

		if optsConf.TestRunID.Valid {
			mi.testRunID = mi.vu.Runtime().ToValue(optsConf.TestRunID.String)
		}
	})

	return mi.testRunID, err
}
