package cloudapi

import (
	"slices"
	"strings"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

// Status represents the status of a test run.
type Status string

func (s Status) String() string { return string(s) }

// Status values for test run status.
const (
	StatusCreated           Status = "created"
	StatusQueued            Status = "queued"
	StatusInitializing      Status = "initializing"
	StatusRunning           Status = "running"
	StatusProcessingMetrics Status = "processing_metrics"
	StatusCompleted         Status = "completed"
	StatusAborted           Status = "aborted"
)

// Result represents the result of a test run.
type Result string

func (r Result) String() string { return string(r) }

// Result values for test run result.
const (
	ResultPassed Result = "passed"
	ResultFailed Result = "failed"
	ResultError  Result = "error"
)

// TestProgress holds the progress of a test run.
type TestProgress struct {
	Status            Status
	Result            Result
	EstimatedDuration int32
	ExecutionDuration int32
	StatusHistory     []StatusEvent
}

// Progress returns the fraction of execution completed, from 0 to 1.
// While running, it uses the status history's running entry timestamp
// for smooth interpolation between server polls.
func (tp *TestProgress) Progress() float64 {
	if tp.IsFinished() || tp.Status == StatusProcessingMetrics {
		return 1
	}
	if tp.Estimated() <= 0 {
		return 0
	}
	if !tp.IsRunning() {
		return 0
	}
	for _, e := range tp.StatusHistory {
		if e.Status == StatusRunning {
			return min(float64(time.Since(e.Entered))/float64(tp.Estimated()), 1)
		}
	}
	return 0
}

// FormatStatus returns a human-readable status string.
func (tp *TestProgress) FormatStatus() string {
	if tp.AbortedByUser() {
		return "Aborted (by user)"
	}
	if tp.Status == StatusCompleted {
		return "Finished" // to match old k6 v1 behavior
	}
	ss := strings.Split(tp.Status.String(), "_")
	for i, s := range ss {
		ss[i] = strings.ToUpper(s[:1]) + s[1:]
	}
	return strings.Join(ss, " ")
}

// Estimated returns the estimated total duration.
func (tp *TestProgress) Estimated() time.Duration {
	return time.Duration(tp.EstimatedDuration) * time.Second
}

// Elapsed returns the time spent running, derived from Progress and Estimated.
func (tp *TestProgress) Elapsed() time.Duration {
	return time.Duration(tp.Progress() * float64(tp.Estimated()))
}

// IsRunning returns true if the test run is currently running.
func (tp *TestProgress) IsRunning() bool {
	return tp.Status == StatusRunning
}

// IsFinished returns true if the current status is a terminal state.
func (tp *TestProgress) IsFinished() bool {
	if tp.Status == StatusCompleted {
		return true
	}
	return tp.Status == StatusAborted
}

// AbortedByUser returns true if the test was aborted by user.
func (tp *TestProgress) AbortedByUser() bool {
	return slices.ContainsFunc(tp.StatusHistory, func(e StatusEvent) bool {
		return e.Status == StatusAborted && e.ByUser != ""
	})
}

// ThresholdsFailed returns true when the test failed due to thresholds.
func (tp *TestProgress) ThresholdsFailed() bool {
	return tp.Result == ResultFailed
}

// TestFailed returns true when the test ended with a non-threshold failure.
func (tp *TestProgress) TestFailed() bool {
	return tp.Result == ResultError
}

// StatusEvent is one entry in a test run's status history.
type StatusEvent struct {
	Status  Status
	Entered time.Time
	ByUser  string
	Code    int32
	Message string
}

// ToStatusModel encodes status events for the v6 wire format.
func ToStatusModel(ee []StatusEvent) []k6cloud.StatusApiModel {
	m := make([]k6cloud.StatusApiModel, len(ee))
	for i, e := range ee {
		m[i] = *k6cloud.NewStatusApiModel(e.Status.String(), e.Entered)
		if e.ByUser == "" && e.Code == 0 && e.Message == "" {
			continue
		}
		m[i].SetExtra(*k6cloud.NewStatusExtraApiModel(
			*k6cloud.NewNullableString(&e.ByUser),
			*k6cloud.NewNullableString(&e.Message),
			*k6cloud.NewNullableInt32(&e.Code),
		))
	}
	return m
}

// FromStatusModel decodes status events from the v6 wire format.
func FromStatusModel(mm []k6cloud.StatusApiModel) []StatusEvent {
	e := make([]StatusEvent, len(mm))
	for i, m := range mm {
		e[i].Status = Status(m.GetType())
		e[i].Entered = m.GetEntered()
		if extra, ok := m.GetExtraOk(); ok {
			e[i].ByUser = extra.GetByUser()
			e[i].Code = extra.GetCode()
			e[i].Message = extra.GetMessage()
		}
	}
	return e
}
