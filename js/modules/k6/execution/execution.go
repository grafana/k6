/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package execution

import (
	"errors"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the execution module.
	ModuleInstance struct {
		modules.InstanceCore
		obj *goja.Object
	}
)

var (
	_ modules.IsModuleV2 = &RootModule{}
	_ modules.Instance   = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.IsModuleV2 interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m modules.InstanceCore) modules.Instance {
	mi := &ModuleInstance{InstanceCore: m}
	rt := m.GetRuntime()
	o := rt.NewObject()
	defProp := func(name string, newInfo func() (*goja.Object, error)) {
		err := o.DefineAccessorProperty(name, rt.ToValue(func() goja.Value {
			obj, err := newInfo()
			if err != nil {
				common.Throw(rt, err)
			}
			return obj
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, err)
		}
	}
	defProp("scenario", mi.newScenarioInfo)
	defProp("instance", mi.newInstanceInfo)
	defProp("vu", mi.newVUInfo)

	mi.obj = o

	return mi
}

// GetExports returns the exports of the execution module.
func (mi *ModuleInstance) GetExports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

// newScenarioInfo returns a goja.Object with property accessors to retrieve
// information about the scenario the current VU is running in.
func (mi *ModuleInstance) newScenarioInfo() (*goja.Object, error) {
	ctx := mi.GetContext()
	rt := common.GetRuntime(ctx)
	vuState := mi.GetState()
	if vuState == nil {
		return nil, errors.New("getting scenario information in the init context is not supported")
	}
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}
	getScenarioState := func() *lib.ScenarioState {
		ss := lib.GetScenarioState(mi.GetContext())
		if ss == nil {
			common.Throw(rt, errors.New("getting scenario information in the init context is not supported"))
		}
		return ss
	}

	si := map[string]func() interface{}{
		"name": func() interface{} {
			return getScenarioState().Name
		},
		"executor": func() interface{} {
			return getScenarioState().Executor
		},
		"startTime": func() interface{} {
			//nolint:lll
			// Return the timestamp in milliseconds, since that's how JS
			// timestamps usually are:
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/Date#time_value_or_timestamp_number
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/now#return_value
			return getScenarioState().StartTime.UnixNano() / int64(time.Millisecond)
		},
		"progress": func() interface{} {
			p, _ := getScenarioState().ProgressFn()
			return p
		},
		"iterationInInstance": func() interface{} {
			return vuState.GetScenarioLocalVUIter()
		},
		"iterationInTest": func() interface{} {
			return vuState.GetScenarioGlobalVUIter()
		},
		"stage": func() interface{} {
			stage, err := getScenarioState().CurrentStage()
			if err != nil {
				common.Throw(rt, err)
			}
			si := map[string]func() interface{}{
				"number": func() interface{} { return stage.Index },
				"name":   func() interface{} { return stage.Name },
			}
			obj, err := newInfoObj(rt, si)
			if err != nil {
				common.Throw(rt, err)
			}
			return obj
		},
	}

	return newInfoObj(rt, si)
}

// newInstanceInfo returns a goja.Object with property accessors to retrieve
// information about the local instance stats.
func (mi *ModuleInstance) newInstanceInfo() (*goja.Object, error) {
	ctx := mi.GetContext()
	es := lib.GetExecutionState(ctx)
	if es == nil {
		return nil, errors.New("getting instance information in the init context is not supported")
	}

	rt := common.GetRuntime(ctx)
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}

	ti := map[string]func() interface{}{
		"currentTestRunDuration": func() interface{} {
			return float64(es.GetCurrentTestRunDuration()) / float64(time.Millisecond)
		},
		"iterationsCompleted": func() interface{} {
			return es.GetFullIterationCount()
		},
		"iterationsInterrupted": func() interface{} {
			return es.GetPartialIterationCount()
		},
		"vusActive": func() interface{} {
			return es.GetCurrentlyActiveVUsCount()
		},
		"vusInitialized": func() interface{} {
			return es.GetInitializedVUsCount()
		},
	}

	return newInfoObj(rt, ti)
}

// newVUInfo returns a goja.Object with property accessors to retrieve
// information about the currently executing VU.
func (mi *ModuleInstance) newVUInfo() (*goja.Object, error) {
	ctx := mi.GetContext()
	vuState := lib.GetState(ctx)
	if vuState == nil {
		return nil, errors.New("getting VU information in the init context is not supported")
	}

	rt := common.GetRuntime(ctx)
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}

	vi := map[string]func() interface{}{
		"idInInstance":        func() interface{} { return vuState.VUID },
		"idInTest":            func() interface{} { return vuState.VUIDGlobal },
		"iterationInInstance": func() interface{} { return vuState.Iteration },
		"iterationInScenario": func() interface{} {
			return vuState.GetScenarioVUIter()
		},
	}

	return newInfoObj(rt, vi)
}

func newInfoObj(rt *goja.Runtime, props map[string]func() interface{}) (*goja.Object, error) {
	o := rt.NewObject()

	for p, get := range props {
		err := o.DefineAccessorProperty(p, rt.ToValue(get), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}
