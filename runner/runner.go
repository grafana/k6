package runner

import (
	"time"
)

// A single metric for a test execution.
type Metric struct {
	Time     time.Time
	Error    error
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
	Run(filename, src string) <-chan Result
}
