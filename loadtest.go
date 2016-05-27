package speedboat

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

/*func (t *LoadTest) StageAt(d time.Duration) (start time.Duration, stage TestStage, stop bool) {
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
	vus = stage.VUs.Start + int(float64(stage.EndVUs-stage.StartVUs)*percentage)

	return vus, false
}*/
