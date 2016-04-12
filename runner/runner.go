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

// A user-printed log comm.
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

	// Currently active VUs; used to calculate how many VUs to spawn/kill.
	currentVUs := 0

	go func() {
		defer close(ch)
		defer close(vuControl)

		wg := sync.WaitGroup{}
		for vus := range control {
			start := func() {
				wg.Add(1)
				go func() {
					defer func() {
						currentVUs -= 1
						wg.Done()
					}()
					for res := range r.RunVU(vuControl) {
						ch <- res
					}
				}()
			}
			stop := func() {
				vuControl <- true
			}
			scale(currentVUs, vus, start, stop)
		}

		wg.Wait()
	}()

	return ch
}

func scale(from, to int, start, stop func()) {
	delta := to - from

	// Start VUs for positive amounts
	for i := 0; i < delta; i++ {
		start()
	}
	// Stop VUs for negative amounts
	for i := delta; i < 0; i++ {
		stop()
	}
}
