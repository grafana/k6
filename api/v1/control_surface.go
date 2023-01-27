package v1

import (
	"context"

	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
)

// ControlSurface includes the methods the REST API can use to control and
// communicate with the rest of k6.
type ControlSurface struct {
	RunCtx        context.Context
	Samples       chan metrics.SampleContainer
	MetricsEngine *engine.MetricsEngine
	Scheduler     *execution.Scheduler
	RunState      *lib.TestRunState
}
