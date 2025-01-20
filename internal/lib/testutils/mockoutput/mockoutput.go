// Package mockoutput implements a mock output for testing purposes.
package mockoutput

import (
	"go.k6.io/k6/metrics"
)

// New exists so that the usage from tests avoids repetition, i.e. is
// mockoutput.New() instead of &mockoutput.MockOutput{}
func New() *MockOutput {
	return &MockOutput{}
}

// MockOutput can be used in tests to mock an actual output.
type MockOutput struct {
	SampleContainers []metrics.SampleContainer
	Samples          []metrics.Sample

	DescFn  func() string
	StartFn func() error
	StopFn  func() error
}

// AddMetricSamples just saves the results in memory.
func (mo *MockOutput) AddMetricSamples(scs []metrics.SampleContainer) {
	mo.SampleContainers = append(mo.SampleContainers, scs...)
	for _, sc := range scs {
		mo.Samples = append(mo.Samples, sc.GetSamples()...)
	}
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
