// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[Metric] = (*MetricBuilder)(nil)

type MetricBuilder struct {
	internal *Metric
	errors   cog.BuildErrors
}

func NewMetricBuilder() *MetricBuilder {
	resource := NewMetric()
	builder := &MetricBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *MetricBuilder) Build() (Metric, error) {
	if err := builder.internal.Validate(); err != nil {
		return Metric{}, err
	}

	if len(builder.errors) > 0 {
		return Metric{}, cog.MakeBuildErrors("summary.metric", builder.errors)
	}

	return *builder.internal, nil
}

// The type of data the metric contains
func (builder *MetricBuilder) Contains(contains MetricContains /* AnonymousEnumToExplicitType */) *MetricBuilder {
	builder.internal.Contains = contains

	return builder
}

// The metric name
func (builder *MetricBuilder) Name(name string) *MetricBuilder {
	builder.internal.Name = name

	return builder
}

// The metric type
func (builder *MetricBuilder) Type(typeArg MetricType /* AnonymousEnumToExplicitType */) *MetricBuilder {
	builder.internal.Type = typeArg

	return builder
}

func (builder *MetricBuilder) Values(values any /* UndiscriminatedDisjunctionToAny */) *MetricBuilder {
	builder.internal.Values = values

	return builder
}
