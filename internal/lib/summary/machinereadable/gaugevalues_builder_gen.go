// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[GaugeValues] = (*GaugeValuesBuilder)(nil)

type GaugeValuesBuilder struct {
	internal *GaugeValues
	errors   cog.BuildErrors
}

func NewGaugeValuesBuilder() *GaugeValuesBuilder {
	resource := NewGaugeValues()
	builder := &GaugeValuesBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *GaugeValuesBuilder) Build() (GaugeValues, error) {
	if err := builder.internal.Validate(); err != nil {
		return GaugeValues{}, err
	}

	if len(builder.errors) > 0 {
		return GaugeValues{}, cog.MakeBuildErrors("summary.gaugeValues", builder.errors)
	}

	return *builder.internal, nil
}

// Maximum observed value
func (builder *GaugeValuesBuilder) Max(max float64) *GaugeValuesBuilder {
	builder.internal.Max = max

	return builder
}

// Minimum observed value
func (builder *GaugeValuesBuilder) Min(min float64) *GaugeValuesBuilder {
	builder.internal.Min = min

	return builder
}

// Current/final gauge value
func (builder *GaugeValuesBuilder) Value(value float64) *GaugeValuesBuilder {
	builder.internal.Value = value

	return builder
}
