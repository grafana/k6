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

// Package segment exports a JS API for accessing execution segment info.
package segment

import (
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		indexes sharedSegmentedIndexes
	}

	// ModuleInstance represents an instance of the segment module.
	ModuleInstance struct {
		vu            modules.VU
		sharedIndexes *sharedSegmentedIndexes
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{
		indexes: sharedSegmentedIndexes{
			data: make(map[string]*SegmentedIndex),
		},
	}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{
		vu:            vu,
		sharedIndexes: &rm.indexes,
	}
	return mi
}

// Exports returns the exports of the segment module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"SegmentedIndex":       mi.SegmentedIndex,
			"SharedSegmentedIndex": mi.SharedSegmentedIndex,
		},
	}
}

// SegmentedIndex is a JS constructor for a SegmentedIndex.
func (mi *ModuleInstance) SegmentedIndex(call goja.ConstructorCall) *goja.Object {
	state := mi.vu.State()
	rt := mi.vu.Runtime()
	if state == nil {
		common.Throw(rt, fmt.Errorf("getting instance information in the init context is not supported"))
	}

	// TODO: maybe replace with lib.GetExecutionState()?
	tuple, err := lib.NewExecutionTuple(state.Options.ExecutionSegment, state.Options.ExecutionSegmentSequence)
	if err != nil {
		common.Throw(rt, err)
	}

	return rt.ToValue(NewSegmentedIndex(tuple)).ToObject(rt)
}

// SharedSegmentedIndex is a JS constructor for a SharedSegmentedIndex.
func (mi *ModuleInstance) SharedSegmentedIndex(call goja.ConstructorCall) *goja.Object {
	rt := mi.vu.Runtime()
	name := call.Argument(0).String()
	if len(name) == 0 {
		common.Throw(rt, fmt.Errorf("empty name provided to SharedArray's constructor"))
	}
	si, err := mi.sharedIndexes.SegmentedIndex(mi.vu.State(), name)
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(si).ToObject(rt)
}
