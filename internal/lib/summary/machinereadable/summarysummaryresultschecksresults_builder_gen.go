// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[SummarySummaryResultsChecksResults] = (*SummarySummaryResultsChecksResultsBuilder)(nil)

type SummarySummaryResultsChecksResultsBuilder struct {
	internal *SummarySummaryResultsChecksResults
	errors   cog.BuildErrors
}

func NewSummarySummaryResultsChecksResultsBuilder() *SummarySummaryResultsChecksResultsBuilder {
	resource := NewSummarySummaryResultsChecksResults()
	builder := &SummarySummaryResultsChecksResultsBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *SummarySummaryResultsChecksResultsBuilder) Build() (SummarySummaryResultsChecksResults, error) {
	if err := builder.internal.Validate(); err != nil {
		return SummarySummaryResultsChecksResults{}, err
	}

	if len(builder.errors) > 0 {
		return SummarySummaryResultsChecksResults{}, cog.MakeBuildErrors("summary.summarySummaryResultsChecksResults", builder.errors)
	}

	return *builder.internal, nil
}

// Number of times the check failed
func (builder *SummarySummaryResultsChecksResultsBuilder) Fails(fails int64) *SummarySummaryResultsChecksResultsBuilder {
	builder.internal.Fails = fails

	return builder
}

// Check name
func (builder *SummarySummaryResultsChecksResultsBuilder) Name(name string) *SummarySummaryResultsChecksResultsBuilder {
	builder.internal.Name = name

	return builder
}

// Number of times the check passed
func (builder *SummarySummaryResultsChecksResultsBuilder) Passes(passes int64) *SummarySummaryResultsChecksResultsBuilder {
	builder.internal.Passes = passes

	return builder
}
