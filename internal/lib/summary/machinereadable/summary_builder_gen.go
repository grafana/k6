// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[Summary] = (*SummaryBuilder)(nil)

type SummaryBuilder struct {
	internal *Summary
	errors   cog.BuildErrors
}

func NewSummaryBuilder() *SummaryBuilder {
	resource := NewSummary()
	builder := &SummaryBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *SummaryBuilder) Build() (Summary, error) {
	if err := builder.internal.Validate(); err != nil {
		return Summary{}, err
	}

	if len(builder.errors) > 0 {
		return Summary{}, cog.MakeBuildErrors("summary.summary", builder.errors)
	}

	return *builder.internal, nil
}

// Configuration information about the test execution
func (builder *SummaryBuilder) Config(config cog.Builder[SummarySummaryConfig]) *SummaryBuilder {
	configResource, err := config.Build()
	if err != nil {
		builder.errors = append(builder.errors, err.(cog.BuildErrors)...)
		return builder
	}
	builder.internal.Config = configResource

	return builder
}

// Metadata about the summary generation
func (builder *SummaryBuilder) Metadata(metadata cog.Builder[SummarySummaryMetadata]) *SummaryBuilder {
	metadataResource, err := metadata.Build()
	if err != nil {
		builder.errors = append(builder.errors, err.(cog.BuildErrors)...)
		return builder
	}
	builder.internal.Metadata = metadataResource

	return builder
}

// Test execution results data
func (builder *SummaryBuilder) Results(results cog.Builder[SummarySummaryResults]) *SummaryBuilder {
	resultsResource, err := results.Build()
	if err != nil {
		builder.errors = append(builder.errors, err.(cog.BuildErrors)...)
		return builder
	}
	builder.internal.Results = resultsResource

	return builder
}

// Schema version in semver 2.0 format (e.g., '1.0.0')
func (builder *SummaryBuilder) Version(version SemVer) *SummaryBuilder {
	builder.internal.Version = version

	return builder
}
