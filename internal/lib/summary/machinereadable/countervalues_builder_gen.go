// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[CounterValues] = (*CounterValuesBuilder)(nil)

type CounterValuesBuilder struct {
	internal *CounterValues
	errors   cog.BuildErrors
}

func NewCounterValuesBuilder() *CounterValuesBuilder {
	resource := NewCounterValues()
	builder := &CounterValuesBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *CounterValuesBuilder) Build() (CounterValues, error) {
	if err := builder.internal.Validate(); err != nil {
		return CounterValues{}, err
	}

	if len(builder.errors) > 0 {
		return CounterValues{}, cog.MakeBuildErrors("summary.counterValues", builder.errors)
	}

	return *builder.internal, nil
}

// Total count of events
func (builder *CounterValuesBuilder) Count(count float64) *CounterValuesBuilder {
	builder.internal.Count = count

	return builder
}
