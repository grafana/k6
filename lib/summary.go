package lib

import (
	"time"

	"go.k6.io/k6/metrics"
)

// LegacySummary contains all the data the summary handler gets.
type LegacySummary struct {
	Metrics         map[string]*metrics.Metric
	RootGroup       *Group
	TestRunDuration time.Duration // TODO: use lib.ExecutionState-based interface instead?
	NoColor         bool          // TODO: drop this when noColor is part of the (runtime) options
	UIState         UIState
}
