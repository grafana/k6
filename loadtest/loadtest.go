package loadtest

import (
	"time"
)

// A load test is composed of at least one "stage", which controls VU distribution.
type Stage struct {
	Duration time.Duration // Duration of this stage.
	StartVUs int           // Set this many VUs at the start of the stage.
	EndVUs   int           // Ramp until there are this many VUs.
}

type LoadTest struct {
	Stages []Stage // Test stages.
}
