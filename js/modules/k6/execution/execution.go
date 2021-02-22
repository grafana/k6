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

	"go.k6.io/k6/lib"
)

// Execution is a JS module to return information about the execution in progress.
type Execution struct{}

// New returns a pointer to a new Execution.
func New() *Execution {
	return &Execution{}
}

// GetVUStats returns information about the currently executing VU.
func (e *Execution) GetVUStats(ctx context.Context) (map[string]interface{}, error) {
	vuState := lib.GetState(ctx)
	if vuState == nil {
		return nil, errors.New("VU information can only be returned from an exported function")
	}

	scID, _ := vuState.GetScenarioVUID()
	out := map[string]interface{}{
		"id":                vuState.Vu,
		"idScenario":        scID,
		"iteration":         vuState.Iteration,
		"iterationScenario": vuState.GetScenarioVUIter(),
	}

	return out, nil
}

// GetScenarioStats returns information about the currently executing scenario.
func (e *Execution) GetScenarioStats(ctx context.Context) (map[string]interface{}, error) {
	ss := lib.GetScenarioState(ctx)
	if ss == nil {
		return nil, errors.New("scenario information can only be returned from an exported function")
	}

	progress, _ := ss.ProgressFn()
	out := map[string]interface{}{
		"name":      ss.Name,
		"executor":  ss.Executor,
		"startTime": ss.StartTime,
		"progress":  progress,
	}

	return out, nil
}

// GetTestStats returns global test information.
func (e *Execution) GetTestStats(ctx context.Context) (map[string]interface{}, error) {
	es := lib.GetExecutionState(ctx)
	if es == nil {
		return nil, errors.New("test information can only be returned from an exported function")
	}

	out := map[string]interface{}{
		// XXX: For consistency, should this be startTime instead, or startTime
		// in ScenarioStats be converted to duration?
		"duration":              es.GetCurrentTestRunDuration().String(),
		"iterationsCompleted":   es.GetFullIterationCount(),
		"iterationsInterrupted": es.GetPartialIterationCount(),
		"vusActive":             es.GetCurrentlyActiveVUsCount(),
		"vusMax":                es.GetInitializedVUsCount(),
	}

	return out, nil
}
