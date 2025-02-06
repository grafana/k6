package lib

import (
	"errors"
	"time"

	"go.k6.io/k6/metrics"
)

// A SummaryMode specifies the mode of the Summary,
// which defines how the end-of-test summary will be rendered.
type SummaryMode int

// Possible values for SummaryMode.
const (
	SummaryModeCompact = SummaryMode(iota) // Compact mode that only displays the total results.
	SummaryModeFull                        // Extended mode that displays total and  partial results.
	SummaryModeLegacy                      // Legacy mode, used for backwards compatibility.
)

// ErrInvalidSummaryMode indicates the serialized summary mode is invalid.
var ErrInvalidSummaryMode = errors.New("invalid summary mode")

const (
	summaryCompactString = "compact"
	summaryFullString    = "full"
	summaryLegacyString  = "legacy"
)

// MarshalJSON serializes a MetricType as a human readable string.
func (m SummaryMode) MarshalJSON() ([]byte, error) {
	txt, err := m.MarshalText()
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(txt) + `"`), nil
}

// MarshalText serializes a MetricType as a human readable string.
func (m SummaryMode) MarshalText() ([]byte, error) {
	switch m {
	case SummaryModeCompact:
		return []byte(summaryCompactString), nil
	case SummaryModeFull:
		return []byte(summaryFullString), nil
	case SummaryModeLegacy:
		return []byte(summaryLegacyString), nil
	default:
		return nil, ErrInvalidSummaryMode
	}
}

// UnmarshalText deserializes a MetricType from a string representation.
func (m *SummaryMode) UnmarshalText(data []byte) error {
	switch string(data) {
	case summaryCompactString:
		*m = SummaryModeCompact
	case summaryFullString:
		*m = SummaryModeFull
	case summaryLegacyString:
		*m = SummaryModeLegacy
	default:
		return ErrInvalidSummaryMode
	}

	return nil
}

func (m SummaryMode) String() string {
	switch m {
	case SummaryModeCompact:
		return summaryCompactString
	case SummaryModeFull:
		return summaryFullString
	case SummaryModeLegacy:
		return summaryLegacyString
	default:
		return "[INVALID]"
	}
}

// ValidateSummaryMode checks if the provided val is a valid summary mode
func ValidateSummaryMode(val string) (sm SummaryMode, err error) {
	if val == "" {
		return SummaryModeCompact, nil
	}
	if err = sm.UnmarshalText([]byte(val)); err != nil {
		return 0, err
	}
	return
}

// Summary is the data structure that holds all the summary data (thresholds, metrics, checks, etc)
// as well as some other information, like certain rendering options.
type Summary struct {
	SummaryThresholds `js:"thresholds"`
	SummaryGroup
	Scenarios map[string]SummaryGroup

	TestRunDuration time.Duration // TODO: use lib.ExecutionState-based interface instead?
	NoColor         bool          // TODO: drop this when noColor is part of the (runtime) options
	UIState         UIState
}

// NewSummary instantiates a new empty Summary.
func NewSummary() *Summary {
	return &Summary{
		SummaryThresholds: NewSummaryThresholds(),
		SummaryGroup: SummaryGroup{
			Metrics: NewSummaryMetrics(),
			Groups:  make(map[string]SummaryGroup),
		},
		Scenarios: make(map[string]SummaryGroup),
	}
}

// SummaryMetricInfo holds the definition of a metric that will be rendered in the summary,
// including the name of the metric, its type (Counter, Trend, etc.) and what contains (data amounts, times, etc.).
type SummaryMetricInfo struct {
	Name     string
	Type     string
	Contains string
}

// SummaryMetric holds all the information needed to display a metric in the summary,
// including its definition and its values.
type SummaryMetric struct {
	SummaryMetricInfo
	Values map[string]float64
}

// NewSummaryMetricFrom instantiates a new SummaryMetric for a given metrics.Sink and the metric's info.
func NewSummaryMetricFrom(
	info SummaryMetricInfo, sink metrics.Sink,
	testDuration time.Duration, summaryTrendStats []string,
) SummaryMetric {
	getMetricValues := metricValueGetter(summaryTrendStats)

	return SummaryMetric{
		SummaryMetricInfo: info,
		Values:            getMetricValues(sink, testDuration),
	}
}

// SummaryMetrics is a collection of SummaryMetric grouped by section (http, network, etc).
type SummaryMetrics struct {
	// HTTP contains summary data specific to HTTP metrics and is used
	// to produce the summary HTTP subsection's content.
	HTTP map[string]SummaryMetric
	// Execution contains summary data specific to Execution metrics and is used
	// to produce the summary Execution subsection's content.
	Execution map[string]SummaryMetric
	// Network contains summary data specific to Network metrics and is used
	// to produce the summary Network subsection's content.
	Network map[string]SummaryMetric

	Browser map[string]SummaryMetric

	WebVitals map[string]SummaryMetric

	Grpc map[string]SummaryMetric

	WebSocket map[string]SummaryMetric `js:"websocket"`

	// Custom contains user-defined metric results as well as extensions metrics
	Custom map[string]SummaryMetric
}

// NewSummaryMetrics instantiates an empty collection of SummaryMetrics.
func NewSummaryMetrics() SummaryMetrics {
	return SummaryMetrics{
		HTTP:      make(map[string]SummaryMetric),
		Execution: make(map[string]SummaryMetric),
		Network:   make(map[string]SummaryMetric),
		Browser:   make(map[string]SummaryMetric),
		WebVitals: make(map[string]SummaryMetric),
		Grpc:      make(map[string]SummaryMetric),
		WebSocket: make(map[string]SummaryMetric),
		Custom:    make(map[string]SummaryMetric),
	}
}

// SummaryChecksMetrics is the subset of checks-specific metrics.
type SummaryChecksMetrics struct {
	Total   SummaryMetric `js:"checks_total"`
	Success SummaryMetric `js:"checks_succeeded"`
	Fail    SummaryMetric `js:"checks_failed"`
}

// SummaryChecks holds the checks information to be rendered in the summary.
type SummaryChecks struct {
	Metrics       SummaryChecksMetrics
	OrderedChecks []*Check
}

// NewSummaryChecks instantiates an empty set of SummaryChecks.
func NewSummaryChecks() *SummaryChecks {
	initChecksMetricData := func(name string, t metrics.MetricType) SummaryMetric {
		return SummaryMetric{
			SummaryMetricInfo: SummaryMetricInfo{
				Name:     name,
				Type:     t.String(),
				Contains: metrics.Default.String(),
			},
			Values: make(map[string]float64),
		}
	}

	return &SummaryChecks{
		Metrics: SummaryChecksMetrics{
			Total:   initChecksMetricData("checks_total", metrics.Counter),
			Success: initChecksMetricData("checks_succeeded", metrics.Rate),
			Fail:    initChecksMetricData("checks_failed", metrics.Rate),
		},
	}
}

// SummaryThreshold holds the information of a threshold to be rendered in the summary.
type SummaryThreshold struct {
	Source string `js:"source"`
	Ok     bool   `js:"ok"`
}

// MetricThresholds is the collection of SummaryThreshold that belongs to the same metric.
type MetricThresholds struct {
	Metric     SummaryMetric      `js:"metric"`
	Thresholds []SummaryThreshold `js:"thresholds"`
}

// SummaryThresholds is a collection of MetricThresholds that will be rendered in the summary.
type SummaryThresholds map[string]MetricThresholds

// NewSummaryThresholds instantiates an empty collection of SummaryThresholds.
func NewSummaryThresholds() SummaryThresholds {
	thresholds := make(SummaryThresholds)
	return thresholds
}

// SummaryGroup is a group of metrics and subgroups (recursive) that will be rendered in the summary.
type SummaryGroup struct {
	Checks  *SummaryChecks // Not always present, thus we use a pointer.
	Metrics SummaryMetrics
	Groups  map[string]SummaryGroup
}

// NewSummaryGroup instantiates an empty SummaryGroup.
func NewSummaryGroup() SummaryGroup {
	return SummaryGroup{
		Metrics: NewSummaryMetrics(),
		Groups:  make(map[string]SummaryGroup),
	}
}

func metricValueGetter(summaryTrendStats []string) func(metrics.Sink, time.Duration) map[string]float64 {
	trendResolvers, err := metrics.GetResolversForTrendColumns(summaryTrendStats)
	if err != nil {
		panic(err.Error()) // this should have been validated already
	}

	return func(sink metrics.Sink, t time.Duration) (result map[string]float64) {
		switch sink := sink.(type) {
		case *metrics.CounterSink:
			result = sink.Format(t)
			result["rate"] = calculateCounterRate(sink.Value, t)
		case *metrics.GaugeSink:
			result = sink.Format(t)
			result["min"] = sink.Min
			result["max"] = sink.Max
		case *metrics.RateSink:
			result = sink.Format(t)
			result["passes"] = float64(sink.Trues)
			result["fails"] = float64(sink.Total - sink.Trues)
		case *metrics.TrendSink:
			result = make(map[string]float64, len(summaryTrendStats))
			for _, col := range summaryTrendStats {
				result[col] = trendResolvers[col](sink)
			}
		}

		return result
	}
}

func calculateCounterRate(count float64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return count / (float64(duration) / float64(time.Second))
}

// LegacySummary contains all the data the summary handler gets.
type LegacySummary struct {
	Metrics         map[string]*metrics.Metric
	RootGroup       *Group
	TestRunDuration time.Duration // TODO: use lib.ExecutionState-based interface instead?
	NoColor         bool          // TODO: drop this when noColor is part of the (runtime) options
	UIState         UIState
}
