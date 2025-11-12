// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[SummarySummaryResultsChecks] = (*SummarySummaryResultsChecksBuilder)(nil)

type SummarySummaryResultsChecksBuilder struct {
	internal *SummarySummaryResultsChecks
	errors   cog.BuildErrors
}

func NewSummarySummaryResultsChecksBuilder() *SummarySummaryResultsChecksBuilder {
	resource := NewSummarySummaryResultsChecks()
	builder := &SummarySummaryResultsChecksBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *SummarySummaryResultsChecksBuilder) Build() (SummarySummaryResultsChecks, error) {
	if err := builder.internal.Validate(); err != nil {
		return SummarySummaryResultsChecks{}, err
	}

	if len(builder.errors) > 0 {
		return SummarySummaryResultsChecks{}, cog.MakeBuildErrors("summary.summarySummaryResultsChecks", builder.errors)
	}

	return *builder.internal, nil
}

// Array of check-related metrics
func (builder *SummarySummaryResultsChecksBuilder) Metrics(metrics []cog.Builder[Metric]) *SummarySummaryResultsChecksBuilder {
	metricsResources := make([]Metric, 0, len(metrics))
	for _, r1 := range metrics {
		metricsDepth1, err := r1.Build()
		if err != nil {
			builder.errors = append(builder.errors, err.(cog.BuildErrors)...)
			return builder
		}
		metricsResources = append(metricsResources, metricsDepth1)
	}
	builder.internal.Metrics = metricsResources

	return builder
}

// Individual check results in execution order
func (builder *SummarySummaryResultsChecksBuilder) Results(results []cog.Builder[SummarySummaryResultsChecksResults]) *SummarySummaryResultsChecksBuilder {
	resultsResources := make([]SummarySummaryResultsChecksResults, 0, len(results))
	for _, r1 := range results {
		resultsDepth1, err := r1.Build()
		if err != nil {
			builder.errors = append(builder.errors, err.(cog.BuildErrors)...)
			return builder
		}
		resultsResources = append(resultsResources, resultsDepth1)
	}
	builder.internal.Results = resultsResources

	return builder
}
