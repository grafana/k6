package lib

import (
	"errors"
	"time"
	
	"go.k6.io/k6/metrics"
)

// Summary contains all of the data the summary handler gets.
type Summary struct {
	Metrics         map[string]*metrics.Metric
	RootGroup       *Group
	TestRunDuration time.Duration // TODO: use lib.ExecutionState-based interface instead?
	NoColor         bool          // TODO: drop this when noColor is part of the (runtime) options
	UIState         UIState
}

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
