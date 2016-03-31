package loadtest

import (
	"time"
)

// Specification for a VU curve.
type VUSpec struct {
	Start int // Start at this many
	End   int // Interpolate to this many
}

// A load test is composed of at least one "stage", which controls VU distribution.
type Stage struct {
	Duration time.Duration // Duration of this stage.
	VUs      VUSpec        // VU specification
}

// A load test definition.
type LoadTest struct {
	Script string  // Script filename.
	URL    string  // URL for simple tests.
	Stages []Stage // Test stages.
}
