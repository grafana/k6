package lib

import (
	"time"
)

// A load test is composed of at least one "stage", which controls VU distribution.
type TestStage struct {
	Duration time.Duration // Duration of this stage.
	StartVUs int           // VUs at the start of this stage.
	EndVUs   int           // VUs at the end of this stage.
}

// A load test definition.
type Test struct {
	Script string      // Script filename.
	URL    string      // URL for simple tests.
	Stages []TestStage // Test stages.
}

func (t *Test) TotalDuration() time.Duration {
	var total time.Duration
	for _, stage := range t.Stages {
		total += stage.Duration
	}
	return total
}

func (t *Test) VUsAt(at time.Duration) int {
	stageStart := time.Duration(0)
	for _, stage := range t.Stages {
		if stageStart+stage.Duration < at {
			stageStart += stage.Duration
			continue
		}
		progress := float64(at-stageStart) / float64(stage.Duration)
		return stage.StartVUs + int(float64(stage.EndVUs-stage.StartVUs)*progress)
	}
	return 0
}

func (t *Test) MaxVUs() int {
	max := 0
	for _, stage := range t.Stages {
		if stage.StartVUs > max {
			max = stage.StartVUs
		}
		if stage.EndVUs > max {
			max = stage.EndVUs
		}
	}
	return max
}
