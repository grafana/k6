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
}

func (t *LoadTest) Load() error {
	srcb, err := ioutil.ReadFile(t.Script)
	if err != nil {
		return err
	}
	t.scriptSource = string(srcb)
	return nil
}

func (t *LoadTest) Run(in <-chan message.Message, out chan message.Message, errors <-chan error) (<-chan message.Message, chan message.Message, <-chan error) {
	oin := make(chan message.Message)

	go func() {
		out <- message.NewToWorker("run.run", message.Fields{
			"filename": t.Script,
			"src":      t.scriptSource,
			"vus":      t.Stages[0].VUs.Start,
		})

		duration := time.Duration(0)
		for i := range t.Stages {
			duration += t.Stages[i].Duration
		}

		timeout := time.After(duration)
	runLoop:
		for {
			select {
			case msg := <-in:
				oin <- msg
			case <-timeout:
				out <- message.NewToWorker("run.stop", message.Fields{})
				close(oin)
				break runLoop
			}
		}
	}()

	return oin, out, errors
}
