package runner

import (
	"sync"
	"time"
)

// A single metric for a test execution.
type Metric struct {
	Start    time.Time
	Duration time.Duration
}

// A user-printed log message.
type LogEntry struct {
	Text string
}

type Runner interface {
	Load(filename, src string) error
	RunVU(duration time.Duration) <-chan interface{}
}

func NewError(err error) interface{} {
	return err
}

func NewLogEntry(text string) interface{} {
	return LogEntry{Text: text}
}

func NewMetric(start time.Time, duration time.Duration) interface{} {
	return Metric{Start: start, Duration: duration}
}

func Run(r Runner, vus int, duration time.Duration) <-chan interface{} {
	ch := make(chan interface{})

	go func() {
		wg := sync.WaitGroup{}
		for i := 0; i < vus; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for res := range r.RunVU(duration) {
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
