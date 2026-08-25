package k6

import (
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

// asyncGroup retains the invocation state used by the settlement handlers.
type asyncGroup struct {
	vu          modules.VU
	start       time.Time
	tagsAndMeta metrics.TagsAndMeta
	// settled prevents duplicate emission and tells Group when a thenable settled synchronously.
	settled bool
}

func (g *asyncGroup) emitDuration() {
	if g.settled {
		return
	}
	g.settled = true

	// Group only creates asyncGroup for an active VU, whose state remains allocated across
	// iterations and cancellation.
	emitGroupDuration(g.vu, g.start, time.Now(), g.tagsAndMeta)
}

func emitGroupDuration(vu modules.VU, start, end time.Time, tagsAndMeta metrics.TagsAndMeta) {
	state := vu.State()
	metrics.PushIfNotDone(vu.Context(), state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GroupDuration,
			Tags:   tagsAndMeta.Tags,
		},
		Time:     end,
		Value:    metrics.D(end.Sub(start)),
		Metadata: tagsAndMeta.Metadata,
	})
}

func (g *asyncGroup) onFulfilled(call sobek.FunctionCall) sobek.Value {
	g.emitDuration()
	return call.Argument(0)
}

func (g *asyncGroup) onRejected(call sobek.FunctionCall) sobek.Value {
	g.emitDuration()
	// Sobek uses a panic to throw an arbitrary JavaScript value from a Go callback. Returning a Go
	// error here would wrap the value and change the promise's rejection reason.
	panic(call.Argument(0))
}

// runAsyncGroup attaches duration emission to a callback's Promise-like result.
func (mi *K6) runAsyncGroup(
	start time.Time,
	tagsAndMeta metrics.TagsAndMeta,
	thenFn sobek.Callable,
	thenable sobek.Value,
) (sobek.Value, bool, error) {
	g := &asyncGroup{
		vu:          mi.vu,
		start:       start,
		tagsAndMeta: tagsAndMeta,
	}
	result, err := g.run(thenFn, thenable)
	return result, g.settled, err
}

func (g *asyncGroup) run(thenFn sobek.Callable, thenable sobek.Value) (sobek.Value, error) {
	rt := g.vu.Runtime()
	return thenFn(
		thenable,
		rt.ToValue(g.onFulfilled),
		rt.ToValue(g.onRejected),
	)
}

// Property access follows JavaScript semantics and may throw.
func asThenable(v sobek.Value) (sobek.Callable, bool) {
	obj, ok := v.(*sobek.Object)
	if !ok {
		return nil, false
	}
	return sobek.AssertFunction(obj.Get("then"))
}
