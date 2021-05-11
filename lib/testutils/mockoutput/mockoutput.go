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

package mockoutput

import (
	"go.k6.io/k6/lib"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

// New exists so that the usage from tests avoids repetition, i.e. is
// mockoutput.New() instead of &mockoutput.MockOutput{}
func New() *MockOutput {
	return &MockOutput{}
}

// MockOutput can be used in tests to mock an actual output.
type MockOutput struct {
	SampleContainers []stats.SampleContainer
	Samples          []stats.Sample
	RunStatus        lib.RunStatus

	DescFn  func() string
	StartFn func() error
	StopFn  func() error
}

var _ output.WithRunStatusUpdates = &MockOutput{}

// AddMetricSamples just saves the results in memory.
func (mo *MockOutput) AddMetricSamples(scs []stats.SampleContainer) {
	mo.SampleContainers = append(mo.SampleContainers, scs...)
	for _, sc := range scs {
		mo.Samples = append(mo.Samples, sc.GetSamples()...)
	}
}

// SetRunStatus updates the RunStatus property.
func (mo *MockOutput) SetRunStatus(latestStatus lib.RunStatus) {
	mo.RunStatus = latestStatus
}

// Description calls the supplied DescFn callback, if available.
func (mo *MockOutput) Description() string {
	if mo.DescFn != nil {
		return mo.DescFn()
	}
	return "mock"
}

// Start calls the supplied StartFn callback, if available.
func (mo *MockOutput) Start() error {
	if mo.StartFn != nil {
		return mo.StartFn()
	}
	return nil
}

// Stop calls the supplied StopFn callback, if available.
func (mo *MockOutput) Stop() error {
	if mo.StopFn != nil {
		return mo.StopFn()
	}
	return nil
}
