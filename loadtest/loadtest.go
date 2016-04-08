package loadtest

import (
	"io/ioutil"
	"path"
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

	Source string // Script source
}

func (t *LoadTest) Load(base string) error {
	srcb, err := ioutil.ReadFile(path.Join(base, t.Script))
	if err != nil {
		return err
	}
	t.Source = string(srcb)
	return nil
}

func (t *LoadTest) StageAt(d time.Duration) (start time.Duration, stage Stage, stop bool) {
	at := time.Duration(0)
	for i := range t.Stages {
		stage = t.Stages[i]
		if d > at+stage.Duration {
			at += stage.Duration
		} else if d < at+stage.Duration {
			return at, stage, false
		}
	}
	return at, stage, true
}

func (t *LoadTest) VUsAt(at time.Duration) (vus int, stop bool) {
	start, stage, stop := t.StageAt(at)
	if stop {
		return 0, true
	}

	stageElapsed := at - start
	percentage := (stageElapsed.Seconds() / stage.Duration.Seconds())
	vus = stage.VUs.Start + int(float64(stage.VUs.End-stage.VUs.Start)*percentage)

	return vus, false
}
