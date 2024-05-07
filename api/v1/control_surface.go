package v1

import (
	"context"

	"github.com/liuxd6825/k6server/execution"
	"github.com/liuxd6825/k6server/lib"
	"github.com/liuxd6825/k6server/metrics"
	"github.com/liuxd6825/k6server/metrics/engine"
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
