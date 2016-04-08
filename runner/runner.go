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
	RunVU(stop <-chan interface{}) <-chan interface{}
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

func Run(r Runner, control <-chan int) <-chan interface{} {
	ch := make(chan interface{})

	// Control channel for VUs; VUs terminate upon reading anything from it, so
	// write to it n times to kill n VUs, close it to kill all of them
	vuControl := make(chan interface{})

	go func() {
		defer close(ch)
		defer close(vuControl)

		wg := sync.WaitGroup{}
		for mod := range control {
			if mod > 0 {
				for i := 0; i < mod; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for res := range r.RunVU(vuControl) {
							ch <- res
						}
					}()
				}
			} else if mod < 0 {
				for i := mod; i < 0; i++ {
					vuControl <- true
				}
			}
		}

		wg.Wait()
	}()

	return ch
}
