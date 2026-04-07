// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Using jennies:
//     GoRawTypes

package summary

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	cog "go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

type CounterValues struct {
	// Total count of events
	Count float64 `json:"count"`
}

// NewCounterValues creates a new CounterValues object.
func NewCounterValues() *CounterValues {
	return &CounterValues{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `CounterValues` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *CounterValues) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "count"
	if fields["count"] != nil {
		if string(fields["count"]) != "null" {
			if err := json.Unmarshal(fields["count"], &resource.Count); err != nil {
				errs = append(errs, cog.MakeBuildErrors("count", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("count", errors.New("required field is null"))...)

		}
		delete(fields, "count")
	} else {
		errs = append(errs, cog.MakeBuildErrors("count", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("CounterValues", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `CounterValues` objects.
func (resource CounterValues) Equals(other CounterValues) bool {
	if resource.Count != other.Count {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `CounterValues` fields for violations and returns them.
func (resource CounterValues) Validate() error {
	var errs cog.BuildErrors
	if !(resource.Count >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"count",
			errors.New("must be >= 0"),
		)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

type GaugeValues struct {
	// Maximum observed value
	Max float64 `json:"max"`
	// Minimum observed value
	Min float64 `json:"min"`
	// Current/final gauge value
	Value float64 `json:"value"`
}

// NewGaugeValues creates a new GaugeValues object.
func NewGaugeValues() *GaugeValues {
	return &GaugeValues{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `GaugeValues` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *GaugeValues) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "max"
	if fields["max"] != nil {
		if string(fields["max"]) != "null" {
			if err := json.Unmarshal(fields["max"], &resource.Max); err != nil {
				errs = append(errs, cog.MakeBuildErrors("max", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("max", errors.New("required field is null"))...)

		}
		delete(fields, "max")
	} else {
		errs = append(errs, cog.MakeBuildErrors("max", errors.New("required field is missing from input"))...)
	}
	// Field "min"
	if fields["min"] != nil {
		if string(fields["min"]) != "null" {
			if err := json.Unmarshal(fields["min"], &resource.Min); err != nil {
				errs = append(errs, cog.MakeBuildErrors("min", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("min", errors.New("required field is null"))...)

		}
		delete(fields, "min")
	} else {
		errs = append(errs, cog.MakeBuildErrors("min", errors.New("required field is missing from input"))...)
	}
	// Field "value"
	if fields["value"] != nil {
		if string(fields["value"]) != "null" {
			if err := json.Unmarshal(fields["value"], &resource.Value); err != nil {
				errs = append(errs, cog.MakeBuildErrors("value", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("value", errors.New("required field is null"))...)

		}
		delete(fields, "value")
	} else {
		errs = append(errs, cog.MakeBuildErrors("value", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("GaugeValues", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `GaugeValues` objects.
func (resource GaugeValues) Equals(other GaugeValues) bool {
	if resource.Max != other.Max {
		return false
	}
	if resource.Min != other.Min {
		return false
	}
	if resource.Value != other.Value {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `GaugeValues` fields for violations and returns them.
func (resource GaugeValues) Validate() error {
	return nil
}

type Metric struct {
	// The type of data the metric contains
	Contains MetricContains/* AnonymousEnumToExplicitType */ `json:"contains"`
	// The metric name
	Name string `json:"name"`
	// The metric type
	Type   MetricType/* AnonymousEnumToExplicitType */ `json:"type"`
	Values any/* UndiscriminatedDisjunctionToAny */ `json:"values"`
}

// NewMetric creates a new Metric object.
func NewMetric() *Metric {
	return &Metric{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `Metric` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *Metric) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "contains"
	if fields["contains"] != nil {
		if string(fields["contains"]) != "null" {
			if err := json.Unmarshal(fields["contains"], &resource.Contains); err != nil {
				errs = append(errs, cog.MakeBuildErrors("contains", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("contains", errors.New("required field is null"))...)

		}
		delete(fields, "contains")
	} else {
		errs = append(errs, cog.MakeBuildErrors("contains", errors.New("required field is missing from input"))...)
	}
	// Field "name"
	if fields["name"] != nil {
		if string(fields["name"]) != "null" {
			if err := json.Unmarshal(fields["name"], &resource.Name); err != nil {
				errs = append(errs, cog.MakeBuildErrors("name", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("name", errors.New("required field is null"))...)

		}
		delete(fields, "name")
	} else {
		errs = append(errs, cog.MakeBuildErrors("name", errors.New("required field is missing from input"))...)
	}
	// Field "type"
	if fields["type"] != nil {
		if string(fields["type"]) != "null" {
			if err := json.Unmarshal(fields["type"], &resource.Type); err != nil {
				errs = append(errs, cog.MakeBuildErrors("type", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("type", errors.New("required field is null"))...)

		}
		delete(fields, "type")
	} else {
		errs = append(errs, cog.MakeBuildErrors("type", errors.New("required field is missing from input"))...)
	}
	// Field "values"
	if fields["values"] != nil {
		if string(fields["values"]) != "null" {
			if err := json.Unmarshal(fields["values"], &resource.Values); err != nil {
				errs = append(errs, cog.MakeBuildErrors("values", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("values", errors.New("required field is null"))...)

		}
		delete(fields, "values")
	} else {
		errs = append(errs, cog.MakeBuildErrors("values", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("Metric", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `Metric` objects.
func (resource Metric) Equals(other Metric) bool {
	if resource.Contains != other.Contains {
		return false
	}
	if resource.Name != other.Name {
		return false
	}
	if resource.Type != other.Type {
		return false
	}
	// is DeepEqual good enough here?
	if !reflect.DeepEqual(resource.Values, other.Values) {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `Metric` fields for violations and returns them.
func (resource Metric) Validate() error {
	return nil
}

type RateValues struct {
	// Number of 'true' events, i.e. occurrences of the event reflected by the metric
	Matches int64 `json:"matches"`
	// Proportion of true events, calculated true / total (value between 0 and 1)
	Rate float64 `json:"rate"`
	// Total number of events (true and non-true)
	Total int64 `json:"total"`
}

// NewRateValues creates a new RateValues object.
func NewRateValues() *RateValues {
	return &RateValues{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `RateValues` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *RateValues) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "matches"
	if fields["matches"] != nil {
		if string(fields["matches"]) != "null" {
			if err := json.Unmarshal(fields["matches"], &resource.Matches); err != nil {
				errs = append(errs, cog.MakeBuildErrors("matches", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("matches", errors.New("required field is null"))...)

		}
		delete(fields, "matches")
	} else {
		errs = append(errs, cog.MakeBuildErrors("matches", errors.New("required field is missing from input"))...)
	}
	// Field "rate"
	if fields["rate"] != nil {
		if string(fields["rate"]) != "null" {
			if err := json.Unmarshal(fields["rate"], &resource.Rate); err != nil {
				errs = append(errs, cog.MakeBuildErrors("rate", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("rate", errors.New("required field is null"))...)

		}
		delete(fields, "rate")
	} else {
		errs = append(errs, cog.MakeBuildErrors("rate", errors.New("required field is missing from input"))...)
	}
	// Field "total"
	if fields["total"] != nil {
		if string(fields["total"]) != "null" {
			if err := json.Unmarshal(fields["total"], &resource.Total); err != nil {
				errs = append(errs, cog.MakeBuildErrors("total", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("total", errors.New("required field is null"))...)

		}
		delete(fields, "total")
	} else {
		errs = append(errs, cog.MakeBuildErrors("total", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("RateValues", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `RateValues` objects.
func (resource RateValues) Equals(other RateValues) bool {
	if resource.Matches != other.Matches {
		return false
	}
	if resource.Rate != other.Rate {
		return false
	}
	if resource.Total != other.Total {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `RateValues` fields for violations and returns them.
func (resource RateValues) Validate() error {
	var errs cog.BuildErrors
	if !(resource.Matches >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"matches",
			errors.New("must be >= 0"),
		)...)
	}
	if !(resource.Rate >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"rate",
			errors.New("must be >= 0"),
		)...)
	}
	if !(resource.Rate <= 1) {
		errs = append(errs, cog.MakeBuildErrors(
			"rate",
			errors.New("must be <= 1"),
		)...)
	}
	if !(resource.Total >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"total",
			errors.New("must be >= 0"),
		)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

type SemVer string

type TrendValues struct {
	// Average (mean) value
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Avg *float64 `json:"avg,omitempty"`
	// Maximum value
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Max *float64 `json:"max,omitempty"`
	// Median value
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Med *float64 `json:"med,omitempty"`
	// Minimum value
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Min *float64 `json:"min,omitempty"`
	// 90th percentile
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	P90 *float64 `json:"p(90),omitempty"`
	// 95th percentile
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	P95 *float64 `json:"p(95),omitempty"`
}

// NewTrendValues creates a new TrendValues object.
func NewTrendValues() *TrendValues {
	return &TrendValues{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `TrendValues` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *TrendValues) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "avg"
	if fields["avg"] != nil {
		if string(fields["avg"]) != "null" {
			if err := json.Unmarshal(fields["avg"], &resource.Avg); err != nil {
				errs = append(errs, cog.MakeBuildErrors("avg", err)...)
			}

		}
		delete(fields, "avg")

	}
	// Field "max"
	if fields["max"] != nil {
		if string(fields["max"]) != "null" {
			if err := json.Unmarshal(fields["max"], &resource.Max); err != nil {
				errs = append(errs, cog.MakeBuildErrors("max", err)...)
			}

		}
		delete(fields, "max")

	}
	// Field "med"
	if fields["med"] != nil {
		if string(fields["med"]) != "null" {
			if err := json.Unmarshal(fields["med"], &resource.Med); err != nil {
				errs = append(errs, cog.MakeBuildErrors("med", err)...)
			}

		}
		delete(fields, "med")

	}
	// Field "min"
	if fields["min"] != nil {
		if string(fields["min"]) != "null" {
			if err := json.Unmarshal(fields["min"], &resource.Min); err != nil {
				errs = append(errs, cog.MakeBuildErrors("min", err)...)
			}

		}
		delete(fields, "min")

	}
	// Field "p(90)"
	if fields["p(90)"] != nil {
		if string(fields["p(90)"]) != "null" {
			if err := json.Unmarshal(fields["p(90)"], &resource.P90); err != nil {
				errs = append(errs, cog.MakeBuildErrors("p(90)", err)...)
			}

		}
		delete(fields, "p(90)")

	}
	// Field "p(95)"
	if fields["p(95)"] != nil {
		if string(fields["p(95)"]) != "null" {
			if err := json.Unmarshal(fields["p(95)"], &resource.P95); err != nil {
				errs = append(errs, cog.MakeBuildErrors("p(95)", err)...)
			}

		}
		delete(fields, "p(95)")

	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("TrendValues", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `TrendValues` objects.
func (resource TrendValues) Equals(other TrendValues) bool {
	if resource.Avg == nil && other.Avg != nil || resource.Avg != nil && other.Avg == nil {
		return false
	}

	if resource.Avg != nil {
		if *resource.Avg != *other.Avg {
			return false
		}
	}
	if resource.Max == nil && other.Max != nil || resource.Max != nil && other.Max == nil {
		return false
	}

	if resource.Max != nil {
		if *resource.Max != *other.Max {
			return false
		}
	}
	if resource.Med == nil && other.Med != nil || resource.Med != nil && other.Med == nil {
		return false
	}

	if resource.Med != nil {
		if *resource.Med != *other.Med {
			return false
		}
	}
	if resource.Min == nil && other.Min != nil || resource.Min != nil && other.Min == nil {
		return false
	}

	if resource.Min != nil {
		if *resource.Min != *other.Min {
			return false
		}
	}
	if resource.P90 == nil && other.P90 != nil || resource.P90 != nil && other.P90 == nil {
		return false
	}

	if resource.P90 != nil {
		if *resource.P90 != *other.P90 {
			return false
		}
	}
	if resource.P95 == nil && other.P95 != nil || resource.P95 != nil && other.P95 == nil {
		return false
	}

	if resource.P95 != nil {
		if *resource.P95 != *other.P95 {
			return false
		}
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `TrendValues` fields for violations and returns them.
func (resource TrendValues) Validate() error {
	return nil
}

type Summary struct {
	// Configuration information about the test execution
	Config SummarySummaryConfig `json:"config"`
	// Metadata about the summary generation
	Metadata SummarySummaryMetadata `json:"metadata"`
	// Test execution results data
	Results SummarySummaryResults `json:"results"`
	// Schema version in semver 2.0 format (e.g., '1.0.0')
	Version SemVer `json:"version"`
}

// NewSummary creates a new Summary object.
func NewSummary() *Summary {
	return &Summary{
		Config:   *NewSummarySummaryConfig(),
		Metadata: *NewSummarySummaryMetadata(),
		Results:  *NewSummarySummaryResults(),
	}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `Summary` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *Summary) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "config"
	if fields["config"] != nil {
		if string(fields["config"]) != "null" {

			resource.Config = SummarySummaryConfig{}
			if err := resource.Config.UnmarshalJSONStrict(fields["config"]); err != nil {
				errs = append(errs, cog.MakeBuildErrors("config", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("config", errors.New("required field is null"))...)

		}
		delete(fields, "config")
	} else {
		errs = append(errs, cog.MakeBuildErrors("config", errors.New("required field is missing from input"))...)
	}
	// Field "metadata"
	if fields["metadata"] != nil {
		if string(fields["metadata"]) != "null" {

			resource.Metadata = SummarySummaryMetadata{}
			if err := resource.Metadata.UnmarshalJSONStrict(fields["metadata"]); err != nil {
				errs = append(errs, cog.MakeBuildErrors("metadata", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("metadata", errors.New("required field is null"))...)

		}
		delete(fields, "metadata")
	} else {
		errs = append(errs, cog.MakeBuildErrors("metadata", errors.New("required field is missing from input"))...)
	}
	// Field "results"
	if fields["results"] != nil {
		if string(fields["results"]) != "null" {

			resource.Results = SummarySummaryResults{}
			if err := resource.Results.UnmarshalJSONStrict(fields["results"]); err != nil {
				errs = append(errs, cog.MakeBuildErrors("results", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("results", errors.New("required field is null"))...)

		}
		delete(fields, "results")
	} else {
		errs = append(errs, cog.MakeBuildErrors("results", errors.New("required field is missing from input"))...)
	}
	// Field "version"
	if fields["version"] != nil {
		if string(fields["version"]) != "null" {
			if err := json.Unmarshal(fields["version"], &resource.Version); err != nil {
				errs = append(errs, cog.MakeBuildErrors("version", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("version", errors.New("required field is null"))...)

		}
		delete(fields, "version")
	} else {
		errs = append(errs, cog.MakeBuildErrors("version", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("Summary", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `Summary` objects.
func (resource Summary) Equals(other Summary) bool {
	if !resource.Config.Equals(other.Config) {
		return false
	}
	if !resource.Metadata.Equals(other.Metadata) {
		return false
	}
	if !resource.Results.Equals(other.Results) {
		return false
	}
	if resource.Version != other.Version {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `Summary` fields for violations and returns them.
func (resource Summary) Validate() error {
	var errs cog.BuildErrors
	if err := resource.Config.Validate(); err != nil {
		errs = append(errs, cog.MakeBuildErrors("config", err)...)
	}
	if err := resource.Metadata.Validate(); err != nil {
		errs = append(errs, cog.MakeBuildErrors("metadata", err)...)
	}
	if err := resource.Results.Validate(); err != nil {
		errs = append(errs, cog.MakeBuildErrors("results", err)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Modified by compiler pass 'AnonymousStructsToNamed'
type SummarySummaryConfig struct {
	// Test run duration in seconds
	Duration float64 `json:"duration"`
	// Type of execution (local or cloud)
	Execution SummarySummaryConfigExecution/* AnonymousEnumToExplicitType */ `json:"execution"`
	// Path or name of the test script
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Script *string `json:"script,omitempty"`
}

// NewSummarySummaryConfig creates a new SummarySummaryConfig object.
func NewSummarySummaryConfig() *SummarySummaryConfig {
	return &SummarySummaryConfig{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `SummarySummaryConfig` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *SummarySummaryConfig) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "duration"
	if fields["duration"] != nil {
		if string(fields["duration"]) != "null" {
			if err := json.Unmarshal(fields["duration"], &resource.Duration); err != nil {
				errs = append(errs, cog.MakeBuildErrors("duration", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("duration", errors.New("required field is null"))...)

		}
		delete(fields, "duration")
	} else {
		errs = append(errs, cog.MakeBuildErrors("duration", errors.New("required field is missing from input"))...)
	}
	// Field "execution"
	if fields["execution"] != nil {
		if string(fields["execution"]) != "null" {
			if err := json.Unmarshal(fields["execution"], &resource.Execution); err != nil {
				errs = append(errs, cog.MakeBuildErrors("execution", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("execution", errors.New("required field is null"))...)

		}
		delete(fields, "execution")
	} else {
		errs = append(errs, cog.MakeBuildErrors("execution", errors.New("required field is missing from input"))...)
	}
	// Field "script"
	if fields["script"] != nil {
		if string(fields["script"]) != "null" {
			if err := json.Unmarshal(fields["script"], &resource.Script); err != nil {
				errs = append(errs, cog.MakeBuildErrors("script", err)...)
			}

		}
		delete(fields, "script")

	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("SummarySummaryConfig", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `SummarySummaryConfig` objects.
func (resource SummarySummaryConfig) Equals(other SummarySummaryConfig) bool {
	if resource.Duration != other.Duration {
		return false
	}
	if resource.Execution != other.Execution {
		return false
	}
	if resource.Script == nil && other.Script != nil || resource.Script != nil && other.Script == nil {
		return false
	}

	if resource.Script != nil {
		if *resource.Script != *other.Script {
			return false
		}
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `SummarySummaryConfig` fields for violations and returns them.
func (resource SummarySummaryConfig) Validate() error {
	var errs cog.BuildErrors
	if !(resource.Duration >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"duration",
			errors.New("must be >= 0"),
		)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Modified by compiler pass 'AnonymousStructsToNamed'
type SummarySummaryMetadata struct {
	// RFC3339 timestamp when summary was generated
	GeneratedAt time.Time `json:"generatedAt"`
	// Version of k6 that generated this summary
	K6Version SemVer `json:"k6Version"`
}

// NewSummarySummaryMetadata creates a new SummarySummaryMetadata object.
func NewSummarySummaryMetadata() *SummarySummaryMetadata {
	return &SummarySummaryMetadata{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `SummarySummaryMetadata` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *SummarySummaryMetadata) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "generatedAt"
	if fields["generatedAt"] != nil {
		if string(fields["generatedAt"]) != "null" {
			if err := json.Unmarshal(fields["generatedAt"], &resource.GeneratedAt); err != nil {
				errs = append(errs, cog.MakeBuildErrors("generatedAt", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("generatedAt", errors.New("required field is null"))...)

		}
		delete(fields, "generatedAt")
	} else {
		errs = append(errs, cog.MakeBuildErrors("generatedAt", errors.New("required field is missing from input"))...)
	}
	// Field "k6Version"
	if fields["k6Version"] != nil {
		if string(fields["k6Version"]) != "null" {
			if err := json.Unmarshal(fields["k6Version"], &resource.K6Version); err != nil {
				errs = append(errs, cog.MakeBuildErrors("k6Version", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("k6Version", errors.New("required field is null"))...)

		}
		delete(fields, "k6Version")
	} else {
		errs = append(errs, cog.MakeBuildErrors("k6Version", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("SummarySummaryMetadata", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `SummarySummaryMetadata` objects.
func (resource SummarySummaryMetadata) Equals(other SummarySummaryMetadata) bool {
	if resource.GeneratedAt != other.GeneratedAt {
		return false
	}
	if resource.K6Version != other.K6Version {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `SummarySummaryMetadata` fields for violations and returns them.
func (resource SummarySummaryMetadata) Validate() error {
	return nil
}

// Modified by compiler pass 'AnonymousStructsToNamed'
type SummarySummaryResultsChecksResults struct {
	// Number of times the check failed
	Fails int64 `json:"fails"`
	// Check name
	Name string `json:"name"`
	// Number of times the check passed
	Passes int64 `json:"passes"`
}

// NewSummarySummaryResultsChecksResults creates a new SummarySummaryResultsChecksResults object.
func NewSummarySummaryResultsChecksResults() *SummarySummaryResultsChecksResults {
	return &SummarySummaryResultsChecksResults{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `SummarySummaryResultsChecksResults` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *SummarySummaryResultsChecksResults) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "fails"
	if fields["fails"] != nil {
		if string(fields["fails"]) != "null" {
			if err := json.Unmarshal(fields["fails"], &resource.Fails); err != nil {
				errs = append(errs, cog.MakeBuildErrors("fails", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("fails", errors.New("required field is null"))...)

		}
		delete(fields, "fails")
	} else {
		errs = append(errs, cog.MakeBuildErrors("fails", errors.New("required field is missing from input"))...)
	}
	// Field "name"
	if fields["name"] != nil {
		if string(fields["name"]) != "null" {
			if err := json.Unmarshal(fields["name"], &resource.Name); err != nil {
				errs = append(errs, cog.MakeBuildErrors("name", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("name", errors.New("required field is null"))...)

		}
		delete(fields, "name")
	} else {
		errs = append(errs, cog.MakeBuildErrors("name", errors.New("required field is missing from input"))...)
	}
	// Field "passes"
	if fields["passes"] != nil {
		if string(fields["passes"]) != "null" {
			if err := json.Unmarshal(fields["passes"], &resource.Passes); err != nil {
				errs = append(errs, cog.MakeBuildErrors("passes", err)...)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("passes", errors.New("required field is null"))...)

		}
		delete(fields, "passes")
	} else {
		errs = append(errs, cog.MakeBuildErrors("passes", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("SummarySummaryResultsChecksResults", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `SummarySummaryResultsChecksResults` objects.
func (resource SummarySummaryResultsChecksResults) Equals(other SummarySummaryResultsChecksResults) bool {
	if resource.Fails != other.Fails {
		return false
	}
	if resource.Name != other.Name {
		return false
	}
	if resource.Passes != other.Passes {
		return false
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `SummarySummaryResultsChecksResults` fields for violations and returns them.
func (resource SummarySummaryResultsChecksResults) Validate() error {
	var errs cog.BuildErrors
	if !(resource.Fails >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"fails",
			errors.New("must be >= 0"),
		)...)
	}
	if !(resource.Passes >= 0) {
		errs = append(errs, cog.MakeBuildErrors(
			"passes",
			errors.New("must be >= 0"),
		)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Modified by compiler pass 'AnonymousStructsToNamed'
type SummarySummaryResultsChecks struct {
	// Array of check-related metrics
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Metrics []Metric `json:"metrics,omitempty"`
	// Individual check results in execution order
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Results []SummarySummaryResultsChecksResults `json:"results,omitempty"`
}

// NewSummarySummaryResultsChecks creates a new SummarySummaryResultsChecks object.
func NewSummarySummaryResultsChecks() *SummarySummaryResultsChecks {
	return &SummarySummaryResultsChecks{}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `SummarySummaryResultsChecks` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *SummarySummaryResultsChecks) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "metrics"
	if fields["metrics"] != nil {
		if string(fields["metrics"]) != "null" {

			partialArray := []json.RawMessage{}
			if err := json.Unmarshal(fields["metrics"], &partialArray); err != nil {
				return err
			}

			for i1 := range partialArray {
				var result1 Metric

				result1 = Metric{}
				if err := result1.UnmarshalJSONStrict(partialArray[i1]); err != nil {
					errs = append(errs, cog.MakeBuildErrors("metrics["+strconv.Itoa(i1)+"]", err)...)
				}
				resource.Metrics = append(resource.Metrics, result1)
			}

		}
		delete(fields, "metrics")

	}
	// Field "results"
	if fields["results"] != nil {
		if string(fields["results"]) != "null" {

			partialArray := []json.RawMessage{}
			if err := json.Unmarshal(fields["results"], &partialArray); err != nil {
				return err
			}

			for i1 := range partialArray {
				var result1 SummarySummaryResultsChecksResults

				result1 = SummarySummaryResultsChecksResults{}
				if err := result1.UnmarshalJSONStrict(partialArray[i1]); err != nil {
					errs = append(errs, cog.MakeBuildErrors("results["+strconv.Itoa(i1)+"]", err)...)
				}
				resource.Results = append(resource.Results, result1)
			}

		}
		delete(fields, "results")

	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("SummarySummaryResultsChecks", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `SummarySummaryResultsChecks` objects.
func (resource SummarySummaryResultsChecks) Equals(other SummarySummaryResultsChecks) bool {

	if len(resource.Metrics) != len(other.Metrics) {
		return false
	}

	for i1 := range resource.Metrics {
		if !resource.Metrics[i1].Equals(other.Metrics[i1]) {
			return false
		}
	}

	if len(resource.Results) != len(other.Results) {
		return false
	}

	for i1 := range resource.Results {
		if !resource.Results[i1].Equals(other.Results[i1]) {
			return false
		}
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `SummarySummaryResultsChecks` fields for violations and returns them.
func (resource SummarySummaryResultsChecks) Validate() error {
	var errs cog.BuildErrors

	for i1 := range resource.Metrics {
		if err := resource.Metrics[i1].Validate(); err != nil {
			errs = append(errs, cog.MakeBuildErrors("metrics["+strconv.Itoa(i1)+"]", err)...)
		}
	}

	for i1 := range resource.Results {
		if err := resource.Results[i1].Validate(); err != nil {
			errs = append(errs, cog.MakeBuildErrors("results["+strconv.Itoa(i1)+"]", err)...)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Modified by compiler pass 'AnonymousStructsToNamed'
type SummarySummaryResults struct {
	// Check execution results
	// Modified by compiler pass 'NotRequiredFieldAsNullableType[nullable=true]'
	Checks *SummarySummaryResultsChecks `json:"checks,omitempty"`
	// Array of all metrics from the test execution
	Metrics []Metric `json:"metrics"`
}

// NewSummarySummaryResults creates a new SummarySummaryResults object.
func NewSummarySummaryResults() *SummarySummaryResults {
	return &SummarySummaryResults{
		Metrics: []Metric{},
	}
}

// UnmarshalJSONStrict implements a custom JSON unmarshalling logic to decode `SummarySummaryResults` from JSON.
// Note: the unmarshalling done by this function is strict. It will fail over required fields being absent from the input, fields having an incorrect type, unexpected fields being present, …
func (resource *SummarySummaryResults) UnmarshalJSONStrict(raw []byte) error {
	if raw == nil {
		return nil
	}
	var errs cog.BuildErrors

	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	// Field "checks"
	if fields["checks"] != nil {
		if string(fields["checks"]) != "null" {

			resource.Checks = &SummarySummaryResultsChecks{}
			if err := resource.Checks.UnmarshalJSONStrict(fields["checks"]); err != nil {
				errs = append(errs, cog.MakeBuildErrors("checks", err)...)
			}

		}
		delete(fields, "checks")

	}
	// Field "metrics"
	if fields["metrics"] != nil {
		if string(fields["metrics"]) != "null" {

			partialArray := []json.RawMessage{}
			if err := json.Unmarshal(fields["metrics"], &partialArray); err != nil {
				return err
			}

			for i1 := range partialArray {
				var result1 Metric

				result1 = Metric{}
				if err := result1.UnmarshalJSONStrict(partialArray[i1]); err != nil {
					errs = append(errs, cog.MakeBuildErrors("metrics["+strconv.Itoa(i1)+"]", err)...)
				}
				resource.Metrics = append(resource.Metrics, result1)
			}
		} else {
			errs = append(errs, cog.MakeBuildErrors("metrics", errors.New("required field is null"))...)

		}
		delete(fields, "metrics")
	} else {
		errs = append(errs, cog.MakeBuildErrors("metrics", errors.New("required field is missing from input"))...)
	}

	for field := range fields {
		errs = append(errs, cog.MakeBuildErrors("SummarySummaryResults", fmt.Errorf("unexpected field '%s'", field))...)
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Equals tests the equality of two `SummarySummaryResults` objects.
func (resource SummarySummaryResults) Equals(other SummarySummaryResults) bool {
	if resource.Checks == nil && other.Checks != nil || resource.Checks != nil && other.Checks == nil {
		return false
	}

	if resource.Checks != nil {
		if !resource.Checks.Equals(*other.Checks) {
			return false
		}
	}

	if len(resource.Metrics) != len(other.Metrics) {
		return false
	}

	for i1 := range resource.Metrics {
		if !resource.Metrics[i1].Equals(other.Metrics[i1]) {
			return false
		}
	}

	return true
}

// Validate checks all the validation constraints that may be defined on `SummarySummaryResults` fields for violations and returns them.
func (resource SummarySummaryResults) Validate() error {
	var errs cog.BuildErrors
	if resource.Checks != nil {
		if err := resource.Checks.Validate(); err != nil {
			errs = append(errs, cog.MakeBuildErrors("checks", err)...)
		}
	}

	for i1 := range resource.Metrics {
		if err := resource.Metrics[i1].Validate(); err != nil {
			errs = append(errs, cog.MakeBuildErrors("metrics["+strconv.Itoa(i1)+"]", err)...)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// Modified by compiler pass 'AnonymousEnumToExplicitType'
// Modified by compiler pass 'PrefixEnumValues'
type MetricContains string

const (
	MetricContainsDefault MetricContains = "default"
	MetricContainsTime    MetricContains = "time"
	MetricContainsData    MetricContains = "data"
)

// Modified by compiler pass 'AnonymousEnumToExplicitType'
// Modified by compiler pass 'PrefixEnumValues'
type MetricType string

const (
	MetricTypeCounter MetricType = "counter"
	MetricTypeGauge   MetricType = "gauge"
	MetricTypeRate    MetricType = "rate"
	MetricTypeTrend   MetricType = "trend"
)

// Modified by compiler pass 'AnonymousEnumToExplicitType'
// Modified by compiler pass 'PrefixEnumValues'
type SummarySummaryConfigExecution string

const (
	SummarySummaryConfigExecutionLocal SummarySummaryConfigExecution = "local"
	SummarySummaryConfigExecutionCloud SummarySummaryConfigExecution = "cloud"
)
