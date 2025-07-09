package summary

import (
	"errors"
	"time"

	"go.k6.io/k6/metrics"
)

// A Mode specifies the mode of the Summary,
// which defines how the end-of-test summary will be rendered.
type Mode int

// Possible values for SummaryMode.
const (
	ModeCompact = Mode(iota) // Compact mode that only displays the total results.
	ModeFull                 // Extended mode that displays total and  partial results.
	ModeLegacy               // Legacy mode, used for backwards compatibility.
)

// ErrInvalidSummaryMode indicates the serialized summary mode is invalid.
var ErrInvalidSummaryMode = errors.New("invalid summary mode")

const (
	compactString = "compact"
	fullString    = "full"
	legacyString  = "legacy"
)

// MarshalJSON serializes a Mode as a human-readable string.
func (m Mode) MarshalJSON() ([]byte, error) {
	txt, err := m.MarshalText()
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(txt) + `"`), nil
}

// MarshalText serializes a Mode as a human-readable string.
func (m Mode) MarshalText() ([]byte, error) {
	switch m {
	case ModeCompact:
		return []byte(compactString), nil
	case ModeFull:
		return []byte(fullString), nil
	case ModeLegacy:
		return []byte(legacyString), nil
	default:
		return nil, ErrInvalidSummaryMode
	}
}

// UnmarshalText deserializes a Mode from a string representation.
func (m *Mode) UnmarshalText(data []byte) error {
	switch string(data) {
	case compactString:
		*m = ModeCompact
	case fullString:
		*m = ModeFull
	case legacyString:
		*m = ModeLegacy
	default:
		return ErrInvalidSummaryMode
	}

	return nil
}

// String returns a human-readable string representation of a Mode.
func (m Mode) String() string {
	switch m {
	case ModeCompact:
		return compactString
	case ModeFull:
		return fullString
	case ModeLegacy:
		return legacyString
	default:
		return "[INVALID]"
	}
}

// ValidateMode checks if the provided val is a valid Mode.
func ValidateMode(val string) (m Mode, err error) {
	if val == "" {
		return ModeCompact, nil
	}
	if err = m.UnmarshalText([]byte(val)); err != nil {
		return 0, err
	}
	return
}

// Summary is the data structure that holds all the summary data (thresholds, metrics, checks, etc)
// as well as some other information, like certain rendering options.
type Summary struct {
	Thresholds `js:"thresholds"`
	Group      `js:"root_group"`
	Scenarios  map[string]Group

	TestRunDuration time.Duration
	NoColor         bool // TODO: drop this when noColor is part of the (runtime) options
	EnableColors    bool
}

// New instantiates a new empty Summary.
func New() *Summary {
	return &Summary{
		Thresholds: NewThresholds(),
		Group: Group{
			Metrics: NewMetrics(),
			Groups:  make(map[string]Group),
		},
		Scenarios: make(map[string]Group),
	}
}

// MetricInfo holds the definition of a metric that will be rendered in the summary,
// including the name of the metric, its type (Counter, Trend, etc.) and what contains (data amounts, times, etc.).
type MetricInfo struct {
	Name     string
	Type     string
	Contains string
}

// Metric holds all the information needed to display a metric in the summary,
// including its definition and its values.
type Metric struct {
	MetricInfo
	Values map[string]float64
}

// NewMetricFrom instantiates a new Metric for a given metrics.Sink and the metric's info.
func NewMetricFrom(info MetricInfo, values map[string]float64) Metric {
	return Metric{
		MetricInfo: info,
		Values:     values,
	}
}

// Metrics is a collection of Metric grouped by section (http, network, etc).
type Metrics struct {
	// HTTP contains summary data specific to HTTP metrics and is used
	// to produce the summary HTTP subsection's content.
	HTTP map[string]Metric
	// Execution contains summary data specific to Execution metrics and is used
	// to produce the summary Execution subsection's content.
	Execution map[string]Metric
	// Network contains summary data specific to Network metrics and is used
	// to produce the summary Network subsection's content.
	Network map[string]Metric

	Browser map[string]Metric

	WebVitals map[string]Metric

	Grpc map[string]Metric

	WebSocket map[string]Metric `js:"websocket"`

	// Custom contains user-defined metric results as well as extensions metrics
	Custom map[string]Metric
}

// NewMetrics instantiates an empty collection of Metrics.
func NewMetrics() Metrics {
	return Metrics{
		HTTP:      make(map[string]Metric),
		Execution: make(map[string]Metric),
		Network:   make(map[string]Metric),
		Browser:   make(map[string]Metric),
		WebVitals: make(map[string]Metric),
		Grpc:      make(map[string]Metric),
		WebSocket: make(map[string]Metric),
		Custom:    make(map[string]Metric),
	}
}

// ChecksMetrics is the subset of checks-specific metrics.
type ChecksMetrics struct {
	Total   Metric `js:"checks_total"`
	Success Metric `js:"checks_succeeded"`
	Fail    Metric `js:"checks_failed"`
}

// Check holds the information to be rendered in the summary for a single check.
type Check struct {
	Name   string `js:"name"`
	Passes int64  `js:"passes"`
	Fails  int64  `js:"fails"`
}

// Checks holds the checks to be rendered in the summary.
type Checks struct {
	Metrics       ChecksMetrics
	OrderedChecks []*Check
}

// NewChecks instantiates an empty set of Checks.
func NewChecks() *Checks {
	initChecksMetricData := func(name string, t metrics.MetricType) Metric {
		return Metric{
			MetricInfo: MetricInfo{
				Name:     name,
				Type:     t.String(),
				Contains: metrics.Default.String(),
			},
			Values: make(map[string]float64),
		}
	}

	return &Checks{
		Metrics: ChecksMetrics{
			Total:   initChecksMetricData("checks_total", metrics.Counter),
			Success: initChecksMetricData("checks_succeeded", metrics.Rate),
			Fail:    initChecksMetricData("checks_failed", metrics.Rate),
		},
	}
}

// Threshold holds the information of a threshold to be rendered in the summary.
type Threshold struct {
	Source string `js:"source"`
	Ok     bool   `js:"ok"`
}

// MetricThresholds is the collection of Threshold that belongs to the same metric.
type MetricThresholds struct {
	Metric     Metric      `js:"metric"`
	Thresholds []Threshold `js:"thresholds"`
}

// Thresholds is a collection of MetricThresholds that will be rendered in the summary.
type Thresholds map[string]MetricThresholds

// NewThresholds instantiates an empty collection of Thresholds.
func NewThresholds() Thresholds {
	thresholds := make(Thresholds)
	return thresholds
}

// Group is a group of metrics and subgroups (recursive) that will be rendered in the summary.
type Group struct {
	Checks      *Checks // Not always present, thus we use a pointer.
	Metrics     Metrics
	Groups      map[string]Group
	GroupsOrder []string // Groups names with the order to be displayed in the summary. Typically same as in code.
}

// NewGroup instantiates an empty Group.
func NewGroup() Group {
	return Group{
		Metrics:     NewMetrics(),
		Groups:      make(map[string]Group),
		GroupsOrder: make([]string, 0),
	}
}
