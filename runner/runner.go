package runner

import (
	"sync"
	"time"
)

// A single metric for a test execution.
type Metric struct {
	Time     time.Time
	Duration time.Duration
}

// A user-printed log message.
type LogEntry struct {
	Time time.Time
	Text string
}

// An envelope for a result.
type Result struct {
	Type     string
	Error    error
	LogEntry LogEntry
	Metric   Metric
}

type Runner interface {
	Load(filename, src string) error
	RunVU() <-chan Result
}

func Run(r Runner, vus int) <-chan Result {
	ch := make(chan Result)

	go func() {
		wg := sync.WaitGroup{}
		for i := 0; i < vus; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for res := range r.RunVU() {
					ch <- res
				}
			}()
		}

		go func() {
			wg.Wait()
			close(ch)
		}()
	}()

	return ch
}
