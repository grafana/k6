// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoBuilder

package summary

import (
	"time"

	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var _ cog.Builder[SummarySummaryMetadata] = (*SummarySummaryMetadataBuilder)(nil)

type SummarySummaryMetadataBuilder struct {
	internal *SummarySummaryMetadata
	errors   cog.BuildErrors
}

func NewSummarySummaryMetadataBuilder() *SummarySummaryMetadataBuilder {
	resource := NewSummarySummaryMetadata()
	builder := &SummarySummaryMetadataBuilder{
		internal: resource,
		errors:   make(cog.BuildErrors, 0),
	}

	return builder
}

func (builder *SummarySummaryMetadataBuilder) Build() (SummarySummaryMetadata, error) {
	if err := builder.internal.Validate(); err != nil {
		return SummarySummaryMetadata{}, err
	}

	if len(builder.errors) > 0 {
		return SummarySummaryMetadata{}, cog.MakeBuildErrors("summary.summarySummaryMetadata", builder.errors)
	}

	return *builder.internal, nil
}

// RFC3339 timestamp when summary was generated
func (builder *SummarySummaryMetadataBuilder) GeneratedAt(generatedAt time.Time) *SummarySummaryMetadataBuilder {
	builder.internal.GeneratedAt = generatedAt

	return builder
}

// Version of k6 that generated this summary
func (builder *SummarySummaryMetadataBuilder) K6Version(k6Version SemVer) *SummarySummaryMetadataBuilder {
	builder.internal.K6Version = k6Version

	return builder
}
