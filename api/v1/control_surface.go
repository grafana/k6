package v1

import (
	"context"

	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/metrics/engine"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
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
