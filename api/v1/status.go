/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package v1

import (
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/core"
	"go.k6.io/k6/lib"
)

type Status struct {
	Status lib.ExecutionStatus `json:"status" yaml:"status"`

	Paused  null.Bool `json:"paused" yaml:"paused"`
	VUs     null.Int  `json:"vus" yaml:"vus"`
	VUsMax  null.Int  `json:"vus-max" yaml:"vus-max"`
	Stopped bool      `json:"stopped" yaml:"stopped"`
	Running bool      `json:"running" yaml:"running"`
	Tainted bool      `json:"tainted" yaml:"tainted"`
}

func NewStatus(engine *core.Engine) Status {
	executionState := engine.ExecutionScheduler.GetState()
	return Status{
		Status:  executionState.GetCurrentExecutionStatus(),
		Running: executionState.HasStarted() && !executionState.HasEnded(),
		Paused:  null.BoolFrom(executionState.IsPaused()),
		Stopped: engine.IsStopped(),
		VUs:     null.IntFrom(executionState.GetCurrentlyActiveVUsCount()),
		VUsMax:  null.IntFrom(executionState.GetInitializedVUsCount()),
		Tainted: engine.IsTainted(),
	}
}

// GetName gets the entity name, implementation of the interface for the jsonapi
// Deprecated: use a constant value instead
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
func (s Status) GetName() string {
	return "status"
}

// GetID gets a status ID (there is no real ID for the status)
// Deprecated: use a constant value instead
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
func (s Status) GetID() string {
	return "default"
}

// SetID do nothing, required by the jsonapi interface
func (s Status) SetID(id string) error {
	return nil
}
