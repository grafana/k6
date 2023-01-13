package k6

import (
	"context"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type groupInstance struct {
	*lib.Group
	startTime time.Time
}

func (g *groupInstance) start() {
	g.startTime = time.Now()
}

func (g *groupInstance) finalize(ctx context.Context, state *lib.State) {
	t := time.Now()
	ctm := state.Tags.GetCurrentValues()
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GroupDuration,
			Tags:   ctm.Tags,
		},
		Time:     t,
		Value:    metrics.D(t.Sub(g.startTime)),
		Metadata: ctm.Metadata,
	})
}

func (mi *K6) asyncGroup(name string, fn goja.Callable) (goja.Value, error) {
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	p, res, _ := rt.NewPromise()
	promiseObject := rt.ToValue(p).ToObject(rt)
	baseGroup := state.Group
	then, _ := goja.AssertFunction(promiseObject.Get("then"))
	res(nil)
	return then(promiseObject, rt.ToValue(func(result goja.Value) (goja.Value, error) {
		g, err := baseGroup.Group(name)
		if err != nil {
			return goja.Undefined(), err
		}

		gi := &groupInstance{Group: g}
		mi.groupInstance = gi
		setGroup(g, state)
		gi.start()
		if err != nil {
			return nil, err // actually return a promise ?!?
		}
		return fn(goja.Undefined())
	}))
}

func setGroup(g *lib.Group, state *lib.State) {
	state.Group = g

	if state.Options.SystemTags.Has(metrics.TagGroup) {
		state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, g.Path)
		})
	}
}

func rootGroup(g *lib.Group) *lib.Group {
	for g.Parent != nil {
		g = g.Parent
	}
	return g
}
