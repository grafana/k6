package v1

import (
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/core"
	"go.k6.io/k6/lib"
)

type Status struct {
	Status lib.ExecutionStatus `json:"status" yaml:"status"`

	Paused  null.Bool `json:"paused" yaml:"paused"`
	VUs     null.Int  `json:"vus" yaml:"vus"`
	VUsMax  null.Int  `json:"vus-max" yaml:"vus-max"`
	Stopped bool      `json:"stopped" yaml:"stopped"`
	Running bool      `json:"running" yaml:"running"`
	Tainted bool      `json:"tainted" yaml:"tainted"`
}

func NewStatus(engine *core.Engine) Status {
	executionState := engine.ExecutionScheduler.GetState()
	return Status{
		Status:  executionState.GetCurrentExecutionStatus(),
		Running: executionState.HasStarted() && !executionState.HasEnded(),
		Paused:  null.BoolFrom(executionState.IsPaused()),
		Stopped: engine.IsStopped(),
		VUs:     null.IntFrom(executionState.GetCurrentlyActiveVUsCount()),
		VUsMax:  null.IntFrom(executionState.GetInitializedVUsCount()),
		Tainted: engine.IsTainted(),
	}
}
