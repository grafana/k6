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
	"fmt"
	"reflect"
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

	o, err := newInfoObj(rt, vi)
	if err != nil {
		return o, err
	}

	err = o.Set("tags", rt.NewDynamicObject(&tagsDynamicObject{
		Runtime: rt,
		State:   vuState,
	}))
	return o, err
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

type tagsDynamicObject struct {
	Runtime *goja.Runtime
	State   *lib.State
}

// Get a property value for the key. May return nil if the property does not exist.
func (o *tagsDynamicObject) Get(key string) goja.Value {
	tag, ok := o.State.Tags.Get(key)
	if !ok {
		return nil
	}
	return o.Runtime.ToValue(tag)
}

// Set a property value for the key. It returns true if succeed.
// String, Boolean and Number types are implicitly converted
// to the goja's relative string representation.
// In any other case, if the Throw option is set then an error is raised
// otherwise just a Warning is written.
func (o *tagsDynamicObject) Set(key string, val goja.Value) bool {
	switch val.ExportType().Kind() { //nolint:exhaustive
	case
		reflect.String,
		reflect.Bool,
		reflect.Int64,
		reflect.Float64:

		o.State.Tags.Set(key, val.String())
		return true
	default:
		err := fmt.Errorf("only String, Boolean and Number types are accepted as a Tag value")
		if o.State.Options.Throw.Bool {
			common.Throw(o.Runtime, err)
			return false
		}
		o.State.Logger.Warnf("the execution.vu.tags.Set('%s') operation has been discarded because %s", key, err.Error())
		return false
	}
}

// Has returns true if the property exists.
func (o *tagsDynamicObject) Has(key string) bool {
	_, ok := o.State.Tags.Get(key)
	return ok
}

// Delete deletes the property for the key. It returns true on success (note, that includes missing property).
func (o *tagsDynamicObject) Delete(key string) bool {
	o.State.Tags.Delete(key)
	return true
}

// Keys returns a slice with all existing property keys. The order is not deterministic.
func (o *tagsDynamicObject) Keys() []string {
	if o.State.Tags.Len() < 1 {
		return nil
	}

	tags := o.State.Tags.Clone()
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	return keys
}
