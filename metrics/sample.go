package metrics

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A TimeSeries uniquely identifies the metric and the set of metric tags that a
// Sample (i.e. a metric measurement) has. TimeSeries objects are comparable
// with the == operator and can be used as map indexes.
type TimeSeries struct {
	Metric *Metric
	Tags   *TagSet
}

// A Sample is a single metric measurement at a specific point in time. It can
// have two sets of string key=value metadata:
//   - indexed Tags for low-cardinality data that are part of the TimeSeries
//   - optional non-indexed Metadata that are meant for high-cardinality information
type Sample struct {
	TimeSeries
	Time  time.Time
	Value float64

	// Optional high-cardinality metadata that won't be indexed in atlas.
	//
	// It can be nil if it wasn't explicitly specified, reduce memory
	// allocations and GC overhead.
	Metadata map[string]string
}

// SampleContainer is a simple abstraction that allows sample
// producers to attach extra information to samples they return
type SampleContainer interface {
	GetSamples() []Sample
}

// Samples is just the simplest SampleContainer implementation
// that will be used when there's no need for extra information
type Samples []Sample

// GetSamples just implements the SampleContainer interface
func (s Samples) GetSamples() []Sample {
	return s
}

// ConnectedSampleContainer is an extension of the SampleContainer
// interface that should be implemented when emitted samples
// are connected and share the same time and tags.
type ConnectedSampleContainer interface {
	SampleContainer
	GetTags() *TagSet
	GetTime() time.Time
}

// ConnectedSamples is the simplest ConnectedSampleContainer
// implementation that will be used when there's no need for
// extra information
type ConnectedSamples struct {
	Samples []Sample
	Tags    *TagSet
	Time    time.Time
}

// GetSamples implements the SampleContainer and ConnectedSampleContainer
// interfaces and returns the stored slice with samples.
func (cs ConnectedSamples) GetSamples() []Sample {
	return cs.Samples
}

// GetTags implements ConnectedSampleContainer interface and returns stored tags.
func (cs ConnectedSamples) GetTags() *TagSet {
	return cs.Tags
}

// GetTime implements ConnectedSampleContainer interface and returns stored time.
func (cs ConnectedSamples) GetTime() time.Time {
	return cs.Time
}

// GetSamples implement the ConnectedSampleContainer interface
// for a single Sample, since it's obviously connected with itself :)
func (s Sample) GetSamples() []Sample {
	return []Sample{s}
}

// GetTags implements ConnectedSampleContainer interface
// and returns the sample's tags.
func (s Sample) GetTags() *TagSet {
	return s.Tags
}

// GetTime just implements ConnectedSampleContainer interface
// and returns the sample's time.
func (s Sample) GetTime() time.Time {
	return s.Time
}

// Ensure that interfaces are implemented correctly
var (
	_ SampleContainer = Sample{}
	_ SampleContainer = Samples{}
)

var (
	_ ConnectedSampleContainer = Sample{}
	_ ConnectedSampleContainer = ConnectedSamples{}
)

// GetBufferedSamples will read all present (i.e. buffered or currently being pushed)
// values in the input channel and return them as a slice.
func GetBufferedSamples(input <-chan SampleContainer) (result []SampleContainer) {
	for {
		select {
		case val, ok := <-input:
			if !ok {
				return
			}
			result = append(result, val)
		default:
			return
		}
	}
}

// PushIfNotDone first checks if the supplied context is done and doesn't push
// the sample container if it is.
func PushIfNotDone(ctx context.Context, output chan<- SampleContainer, sample SampleContainer) bool {
	if ctx.Err() != nil {
		return false
	}
	output <- sample
	return true
}

// GetResolversForTrendColumns checks if passed trend columns are valid for use in
// the summary output and then returns a map of the corresponding resolvers.
func GetResolversForTrendColumns(trendColumns []string) (map[string]func(s *TrendSink) float64, error) {
	staticResolvers := map[string]func(s *TrendSink) float64{
		"avg":   func(s *TrendSink) float64 { return s.Avg() },
		"min":   func(s *TrendSink) float64 { return s.Min() },
		"med":   func(s *TrendSink) float64 { return s.P(0.5) },
		"max":   func(s *TrendSink) float64 { return s.Max() },
		"count": func(s *TrendSink) float64 { return float64(s.Count()) },
	}
	dynamicResolver := func(percentile float64) func(s *TrendSink) float64 {
		return func(s *TrendSink) float64 {
			return s.P(percentile / 100)
		}
	}

	result := make(map[string]func(s *TrendSink) float64, len(trendColumns))

	for _, stat := range trendColumns {
		if staticStat, ok := staticResolvers[stat]; ok {
			result[stat] = staticStat
			continue
		}

		percentile, err := parsePercentile(stat)
		if err != nil {
			return nil, err
		}
		result[stat] = dynamicResolver(percentile)
	}

	return result, nil
}

// parsePercentile is a helper function to parse and validate percentile notations
func parsePercentile(stat string) (float64, error) {
	if !strings.HasPrefix(stat, "p(") || !strings.HasSuffix(stat, ")") {
		return 0, fmt.Errorf("invalid trend stat '%s', unknown format", stat)
	}

	percentile, err := strconv.ParseFloat(stat[2:len(stat)-1], 64)

	if err != nil || (percentile < 0) || (percentile > 100) {
		return 0, fmt.Errorf("invalid percentile trend stat value '%s', provide a number between 0 and 100", stat)
	}

	return percentile, nil
}
