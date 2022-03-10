package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mailru/easyjson/jwriter"
)

// A Sample is a single measurement.
type Sample struct {
	Metric *Metric
	Time   time.Time
	Tags   *SampleTags
	Value  float64
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
	GetTags() *SampleTags
	GetTime() time.Time
}

// ConnectedSamples is the simplest ConnectedSampleContainer
// implementation that will be used when there's no need for
// extra information
type ConnectedSamples struct {
	Samples []Sample
	Tags    *SampleTags
	Time    time.Time
}

// GetSamples implements the SampleContainer and ConnectedSampleContainer
// interfaces and returns the stored slice with samples.
func (cs ConnectedSamples) GetSamples() []Sample {
	return cs.Samples
}

// GetTags implements ConnectedSampleContainer interface and returns stored tags.
func (cs ConnectedSamples) GetTags() *SampleTags {
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
func (s Sample) GetTags() *SampleTags {
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
		"avg":   func(s *TrendSink) float64 { return s.Avg },
		"min":   func(s *TrendSink) float64 { return s.Min },
		"med":   func(s *TrendSink) float64 { return s.Med },
		"max":   func(s *TrendSink) float64 { return s.Max },
		"count": func(s *TrendSink) float64 { return float64(s.Count) },
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

// SampleTags is an immutable string[string] map for tags. Once a tag
// set is created, direct modification is prohibited. It has
// copy-on-write semantics and uses pointers for faster comparison
// between maps, since the same tag set is often used for multiple samples.
// All methods should not panic, even if they are called on a nil pointer.
//easyjson:skip
type SampleTags struct {
	tags map[string]string
	json []byte
}

// Get returns an empty string and false if the the requested key is not
// present or its value and true if it is.
func (st *SampleTags) Get(key string) (string, bool) {
	if st == nil {
		return "", false
	}
	val, ok := st.tags[key]
	return val, ok
}

// IsEmpty checks for a nil pointer or zero tags.
// It's necessary because of this envconfig issue: https://github.com/kelseyhightower/envconfig/issues/113
func (st *SampleTags) IsEmpty() bool {
	return st == nil || len(st.tags) == 0
}

// IsEqual tries to compare two tag sets with maximum efficiency.
func (st *SampleTags) IsEqual(other *SampleTags) bool {
	if st == other {
		return true
	}
	if st == nil || other == nil || len(st.tags) != len(other.tags) {
		return false
	}
	for k, v := range st.tags {
		if otherv, ok := other.tags[k]; !ok || v != otherv {
			return false
		}
	}
	return true
}

func (st *SampleTags) Contains(other *SampleTags) bool {
	if st == other || other == nil {
		return true
	}
	if st == nil || len(st.tags) < len(other.tags) {
		return false
	}

	for k, v := range other.tags {
		if myv, ok := st.tags[k]; !ok || myv != v {
			return false
		}
	}

	return true
}

// MarshalJSON serializes SampleTags to a JSON string and caches
// the result. It is not thread safe in the sense that the Go race
// detector will complain if it's used concurrently, but no data
// should be corrupted.
func (st *SampleTags) MarshalJSON() ([]byte, error) {
	if st.IsEmpty() {
		return []byte("null"), nil
	}
	if st.json != nil {
		return st.json, nil
	}
	res, err := json.Marshal(st.tags)
	if err != nil {
		return res, err
	}
	st.json = res
	return res, nil
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (st *SampleTags) MarshalEasyJSON(w *jwriter.Writer) {
	w.RawByte('{')
	first := true
	for k, v := range st.tags {
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(k)
		w.RawByte(':')
		w.String(v)
	}
	w.RawByte('}')
}

// UnmarshalJSON deserializes SampleTags from a JSON string.
func (st *SampleTags) UnmarshalJSON(data []byte) error {
	if st == nil {
		*st = SampleTags{}
	}
	return json.Unmarshal(data, &st.tags)
}

// CloneTags copies the underlying set of a sample tags and
// returns it. If the receiver is nil, it returns an empty non-nil map.
func (st *SampleTags) CloneTags() map[string]string {
	if st == nil {
		return map[string]string{}
	}
	res := make(map[string]string, len(st.tags))
	for k, v := range st.tags {
		res[k] = v
	}
	return res
}

// NewSampleTags *copies* the supplied tag set and returns a new SampleTags
// instance with the key-value pairs from it.
func NewSampleTags(data map[string]string) *SampleTags {
	if len(data) == 0 {
		return nil
	}

	tags := map[string]string{}
	for k, v := range data {
		tags[k] = v
	}
	return &SampleTags{tags: tags}
}

// IntoSampleTags "consumes" the passed map and creates a new SampleTags
// struct with the data. The map is set to nil as a hint that it shouldn't
// be changed after it has been transformed into an "immutable" tag set.
// Oh, how I miss Rust and move semantics... :)
func IntoSampleTags(data *map[string]string) *SampleTags {
	if len(*data) == 0 {
		return nil
	}

	res := SampleTags{tags: *data}
	*data = nil
	return &res
}
