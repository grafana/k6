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

type Runner interface {
	Load(filename, src string) error
	RunVU() <-chan interface{}
}

func NewError(err error) interface{} {
	return err
}

func NewLogEntry(entry LogEntry) interface{} {
	return entry
}

func NewMetric(metric Metric) interface{} {
	return metric
}

func Run(r Runner, vus int) <-chan interface{} {
	ch := make(chan interface{})

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
