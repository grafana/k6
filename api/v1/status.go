package v1

import (
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
)

// Status represents the current status of the test run.
type Status struct {
	Status lib.ExecutionStatus `json:"status" yaml:"status"`

	Paused          null.Bool        `json:"paused" yaml:"paused"`
	VUs             null.Int         `json:"vus" yaml:"vus"`
	VUsMax          null.Int         `json:"vus-max" yaml:"vus-max"`
	Stopped         bool             `json:"stopped" yaml:"stopped"`
	Running         bool             `json:"running" yaml:"running"`
	Tainted         bool             `json:"tainted" yaml:"tainted"`
	ExecutionResult *ExecutionResult `json:"execution_result" yaml:"execution_result"`
}

// ExecutionResult represents the result of a completed test execution.
type ExecutionResult struct {
	ExitCode int `json:"exit_code" yaml:"exit_code"`
}

func newStatus(cs *ControlSurface) Status {
	executionState := cs.Scheduler.GetState()
	var executionResult *ExecutionResult
	if exitCode, ok := executionState.GetExecutionResult(); ok {
		executionResult = &ExecutionResult{ExitCode: exitCode}
	}
	isStopped := false
	select {
	case <-cs.RunCtx.Done():
		isStopped = true
	default:
	}
	return Status{
		Status:          executionState.GetCurrentExecutionStatus(),
		Running:         executionState.HasStarted() && !executionState.HasEnded(),
		Paused:          null.BoolFrom(executionState.IsPaused()),
		Stopped:         isStopped,
		VUs:             null.IntFrom(executionState.GetCurrentlyActiveVUsCount()),
		VUsMax:          null.IntFrom(executionState.GetInitializedVUsCount()),
		Tainted:         cs.MetricsEngine.GetMetricsWithBreachedThresholdsCount() > 0,
		ExecutionResult: executionResult,
	}
}
