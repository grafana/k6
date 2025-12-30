// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[TrendValues] = (*TrendValuesBuilder)(nil)

type TrendValuesBuilder struct {
	internal *TrendValues
	errors   cog.BuildErrors
}

func NewTrendValuesBuilder() *TrendValuesBuilder {
	resource := NewTrendValues()
	builder := &TrendValuesBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *TrendValuesBuilder) Build() (TrendValues, error) {
	if err := builder.internal.Validate(); err != nil {
		return TrendValues{}, err
	}

	if len(builder.errors) > 0 {
		return TrendValues{}, cog.MakeBuildErrors("summary.trendValues", builder.errors)
	}

	return *builder.internal, nil
}

// Average (mean) value
func (builder *TrendValuesBuilder) Avg(avg float64) *TrendValuesBuilder {
	builder.internal.Avg = &avg

	return builder
}

// Maximum value
func (builder *TrendValuesBuilder) Max(max float64) *TrendValuesBuilder {
	builder.internal.Max = &max

	return builder
}

// Median value
func (builder *TrendValuesBuilder) Med(med float64) *TrendValuesBuilder {
	builder.internal.Med = &med

	return builder
}

// Minimum value
func (builder *TrendValuesBuilder) Min(min float64) *TrendValuesBuilder {
	builder.internal.Min = &min

	return builder
}

// 90th percentile
func (builder *TrendValuesBuilder) P90(p90 float64) *TrendValuesBuilder {
	builder.internal.P90 = &p90

	return builder
}

// 95th percentile
func (builder *TrendValuesBuilder) P95(p95 float64) *TrendValuesBuilder {
	builder.internal.P95 = &p95

	return builder
}
