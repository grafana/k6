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
	"context"
	"errors"

	"github.com/dop251/goja"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
)

// Execution is a JS module to return information about the execution in progress.
type Execution struct{}

// New returns a pointer to a new Execution.
func New() *Execution {
	return &Execution{}
}

// GetVUStats returns information about the currently executing VU.
func (e *Execution) GetVUStats(ctx context.Context) (goja.Value, error) {
	vuState := lib.GetState(ctx)
	if vuState == nil {
		return nil, errors.New("getting VU information in the init context is not supported")
	}

	rt := common.GetRuntime(ctx)
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}

	stats := map[string]interface{}{
		"id":         vuState.Vu,
		"idScenario": vuState.VUIDScenario,
		"iteration":  vuState.Iteration,
		"iterationScenario": func() goja.Value {
			return rt.ToValue(vuState.GetScenarioVUIter())
		},
	}

	obj, err := newLazyJSObject(rt, stats)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// GetScenarioStats returns information about the currently executing scenario.
func (e *Execution) GetScenarioStats(ctx context.Context) (goja.Value, error) {
	ss := lib.GetScenarioState(ctx)
	vuState := lib.GetState(ctx)
	if ss == nil || vuState == nil {
		return nil, errors.New("getting scenario information in the init context is not supported")
	}

	rt := common.GetRuntime(ctx)
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}

	var iterGlobal interface{}
	if vuState.GetScenarioGlobalVUIter != nil {
		iterGlobal = vuState.GetScenarioGlobalVUIter()
	} else {
		iterGlobal = goja.Null()
	}

	stats := map[string]interface{}{
		"name":      ss.Name,
		"executor":  ss.Executor,
		"startTime": ss.StartTime,
		"progress": func() goja.Value {
			p, _ := ss.ProgressFn()
			return rt.ToValue(p)
		},
		"iteration":       vuState.GetScenarioLocalVUIter(),
		"iterationGlobal": iterGlobal,
	}

	obj, err := newLazyJSObject(rt, stats)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// GetTestStats returns global test information.
func (e *Execution) GetTestStats(ctx context.Context) (goja.Value, error) {
	es := lib.GetExecutionState(ctx)
	if es == nil {
		return nil, errors.New("getting test information in the init context is not supported")
	}

	rt := common.GetRuntime(ctx)
	if rt == nil {
		return nil, errors.New("goja runtime is nil in context")
	}

	stats := map[string]interface{}{
		// XXX: For consistency, should this be startTime instead, or startTime
		// in ScenarioStats be converted to duration?
		"duration": func() goja.Value {
			return rt.ToValue(es.GetCurrentTestRunDuration().String())
		},
		"iterationsCompleted": func() goja.Value {
			return rt.ToValue(es.GetFullIterationCount())
		},
		"iterationsInterrupted": func() goja.Value {
			return rt.ToValue(es.GetPartialIterationCount())
		},
		"vusActive": func() goja.Value {
			return rt.ToValue(es.GetCurrentlyActiveVUsCount())
		},
		"vusMax": func() goja.Value {
			return rt.ToValue(es.GetInitializedVUsCount())
		},
	}

	obj, err := newLazyJSObject(rt, stats)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func newLazyJSObject(rt *goja.Runtime, data map[string]interface{}) (goja.Value, error) {
	obj := rt.NewObject()

	for k, v := range data {
		if val, ok := v.(func() goja.Value); ok {
			if err := obj.DefineAccessorProperty(k, rt.ToValue(val),
				nil, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
				return nil, err
			}
		} else {
			if err := obj.DefineDataProperty(k, rt.ToValue(v),
				goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
				return nil, err
			}
		}
	}

	return obj, nil
}
