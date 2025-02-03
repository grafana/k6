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
	SummaryModeFull                        // Extended mode that displays the total and also partial (per-group, etc.) results.
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

type Summary struct {
	SummaryThresholds `js:"thresholds"`
	SummaryGroup
	Scenarios map[string]SummaryGroup
}

func NewSummary() Summary {
	return Summary{
		SummaryThresholds: NewSummaryThresholds(),
		SummaryGroup: SummaryGroup{
			Metrics: NewSummaryMetrics(),
			Groups:  make(map[string]SummaryGroup),
		},
		Scenarios: make(map[string]SummaryGroup),
	}
}

type SummaryMetricInfo struct {
	Name     string
	Type     string
	Contains string
}

type SummaryMetric struct {
	SummaryMetricInfo
	Values map[string]float64
}

func NewSummaryMetricFrom(
	info SummaryMetricInfo, sink metrics.Sink,
	testDuration time.Duration, summaryTrendStats []string,
) SummaryMetric {
	// TODO: we obtain this from [options.SummaryTrendStats] which is a string slice
	getMetricValues := metricValueGetter(summaryTrendStats)

	return SummaryMetric{
		SummaryMetricInfo: info,
		Values:            getMetricValues(sink, testDuration),
	}
}

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

type SummaryChecksMetrics struct {
	Total   SummaryMetric `js:"checks_total"`
	Success SummaryMetric `js:"checks_succeeded"`
	Fail    SummaryMetric `js:"checks_failed"`
}

type SummaryChecks struct {
	Metrics       SummaryChecksMetrics
	OrderedChecks []*Check
}

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

type SummaryThreshold struct {
	Source string `js:"source"`
	Ok     bool   `js:"ok"`
}

type MetricThresholds struct {
	Metric     SummaryMetric      `js:"metric"`
	Thresholds []SummaryThreshold `js:"thresholds"`
}

type SummaryThresholds map[string]MetricThresholds

func NewSummaryThresholds() SummaryThresholds {
	thresholds := make(SummaryThresholds)
	return thresholds
}

type SummaryGroup struct {
	Checks  *SummaryChecks // Not always present, thus we use a pointer.
	Metrics SummaryMetrics
	Groups  map[string]SummaryGroup
}

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
