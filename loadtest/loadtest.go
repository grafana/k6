package loadtest

import (
	"github.com/loadimpact/speedboat/message"
	"io/ioutil"
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

	scriptSource string
	currentVUs   int
}

func (t *LoadTest) Load() error {
	srcb, err := ioutil.ReadFile(t.Script)
	if err != nil {
		return err
	}
	t.scriptSource = string(srcb)
	return nil
}

func (t *LoadTest) StageAt(d time.Duration) (stage Stage, stop bool) {
	at := time.Duration(0)
	for i := range t.Stages {
		stage = t.Stages[i]
		if d > at+stage.Duration {
			at += stage.Duration
		} else if d < at+stage.Duration {
			return stage, false
		}
	}
	return stage, true
}

func (t *LoadTest) VUsAt(at time.Duration) (vus int, stop bool) {
	stage, stop := t.StageAt(at)
	if stop {
		return 0, true
	}
	return stage.VUs.Start, false
}

func (t *LoadTest) Run(in <-chan message.Message, out chan message.Message, errors <-chan error) (<-chan message.Message, chan message.Message, <-chan error) {
	oin := make(chan message.Message)

	go func() {
		out <- message.NewToWorker("run.run", message.Fields{
			"filename": t.Script,
			"src":      t.scriptSource,
			"vus":      t.Stages[0].VUs.Start,
		})

		startTime := time.Now()
		intervene := time.Tick(time.Duration(1) * time.Second)
	runLoop:
		for {
			select {
			case msg := <-in:
				oin <- msg
			case <-intervene:
				vus, stop := t.VUsAt(time.Since(startTime))
				if stop {
					out <- message.NewToWorker("run.stop", message.Fields{})
					close(oin)
					break runLoop
				}
				out <- message.NewToWorker("run.vus", message.Fields{
					"vus": vus,
				})
				t.currentVUs = vus
				println(t.currentVUs)
			}
		}
	}()

	return oin, out, errors
}
