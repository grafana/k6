// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[SummarySummaryConfig] = (*SummarySummaryConfigBuilder)(nil)

type SummarySummaryConfigBuilder struct {
	internal *SummarySummaryConfig
	errors   cog.BuildErrors
}

func NewSummarySummaryConfigBuilder() *SummarySummaryConfigBuilder {
	resource := NewSummarySummaryConfig()
	builder := &SummarySummaryConfigBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *SummarySummaryConfigBuilder) Build() (SummarySummaryConfig, error) {
	if err := builder.internal.Validate(); err != nil {
		return SummarySummaryConfig{}, err
	}

	if len(builder.errors) > 0 {
		return SummarySummaryConfig{}, cog.MakeBuildErrors("summary.summarySummaryConfig", builder.errors)
	}

	return *builder.internal, nil
}

// Test run duration in seconds
func (builder *SummarySummaryConfigBuilder) Duration(duration float64) *SummarySummaryConfigBuilder {
	builder.internal.Duration = duration

	return builder
}

// Type of execution (local or cloud)
func (builder *SummarySummaryConfigBuilder) Execution(execution SummarySummaryConfigExecution /* AnonymousEnumToExplicitType */) *SummarySummaryConfigBuilder {
	builder.internal.Execution = execution

	return builder
}

// Path or name of the test script
func (builder *SummarySummaryConfigBuilder) Script(script string) *SummarySummaryConfigBuilder {
	builder.internal.Script = &script

	return builder
}
