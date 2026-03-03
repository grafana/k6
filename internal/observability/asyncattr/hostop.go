package asyncattr

import (
	"context"
	"runtime"
	"runtime/pprof"
	rtrace "runtime/trace"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
)

type contextKey string

const (
	ctxHostOpIDKey contextKey = "k6.asyncattr.host_op_id"
)

var hostOpSeq uint64 //nolint:gochecknoglobals

// RunHostOp executes fn under host-op labels/trace region and attributes
// allocation deltas to the target Sobek runtime.
func RunHostOp[T any](
	ctx context.Context,
	rt *sobek.Runtime,
	kind, name string,
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T
	if rt == nil {
		return fn(ctx)
	}

	opID := atomic.AddUint64(&hostOpSeq, 1)
	lctx := pprof.WithLabels(ctx, pprof.Labels(
		"js.host.op_id", strconv.FormatUint(opID, 10),
		"js.host.kind", kind,
		"js.host.name", name,
	))
	lctx = context.WithValue(lctx, ctxHostOpIDKey, opID)
	pprof.SetGoroutineLabels(lctx)

	var startMS runtime.MemStats
	runtime.ReadMemStats(&startMS)

	start := time.Now()
	var (
		res T
		err error
	)
	rtrace.WithRegion(lctx, "k6.js.host."+kind, func() {
		res, err = fn(lctx)
	})
	rtrace.Log(lctx, "js.host.run_ns", strconv.FormatInt(time.Since(start).Nanoseconds(), 10))

	var stopMS runtime.MemStats
	runtime.ReadMemStats(&stopMS)
	var allocObjects, allocSpace int64
	if stopMS.Mallocs >= startMS.Mallocs {
		allocObjects = int64(stopMS.Mallocs - startMS.Mallocs)
	}
	if stopMS.TotalAlloc >= startMS.TotalAlloc {
		allocSpace = int64(stopMS.TotalAlloc - startMS.TotalAlloc)
	}
	rt.AddPendingAsyncAllocs(allocObjects, allocSpace)

	if err != nil {
		return zero, err
	}
	return res, nil
}
