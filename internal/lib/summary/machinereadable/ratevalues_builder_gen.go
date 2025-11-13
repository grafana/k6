// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[RateValues] = (*RateValuesBuilder)(nil)

type RateValuesBuilder struct {
	internal *RateValues
	errors   cog.BuildErrors
}

func NewRateValuesBuilder() *RateValuesBuilder {
	resource := NewRateValues()
	builder := &RateValuesBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *RateValuesBuilder) Build() (RateValues, error) {
	if err := builder.internal.Validate(); err != nil {
		return RateValues{}, err
	}

	if len(builder.errors) > 0 {
		return RateValues{}, cog.MakeBuildErrors("summary.rateValues", builder.errors)
	}

	return *builder.internal, nil
}

// Number of 'true' events, i.e. occurrences of the event reflected by the metric
func (builder *RateValuesBuilder) Matches(matches int64) *RateValuesBuilder {
	builder.internal.Matches = matches

	return builder
}

// Proportion of true events, calculated true / total (value between 0 and 1)
func (builder *RateValuesBuilder) Rate(rate float64) *RateValuesBuilder {
	builder.internal.Rate = rate

	return builder
}

// Total number of events (true and non-true)
func (builder *RateValuesBuilder) Total(total int64) *RateValuesBuilder {
	builder.internal.Total = total

	return builder
}
