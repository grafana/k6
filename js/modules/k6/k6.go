// Package k6 implements the module imported as 'k6' from inside k6.
package k6

import (
	"errors"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

var (
	// ErrGroupInInitContext is returned when group() are using in the init context.
	ErrGroupInInitContext = common.NewInitContextError("Using group() in the init context is not supported")

	// ErrCheckInInitContext is returned when check() are using in the init context.
	ErrCheckInInitContext = common.NewInitContextError("Using check() in the init context is not supported")
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// K6 represents an instance of the k6 module.
	K6 struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &K6{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &K6{vu: vu}
}

// Exports returns the exports of the k6 module.
func (mi *K6) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"check":      mi.Check,
			"fail":       mi.Fail,
			"group":      mi.Group,
			"randomSeed": mi.RandomSeed,
			"sleep":      mi.Sleep,
		},
	}
}

// Fail is a fancy way of saying `throw "something"`.
func (*K6) Fail(msg string) (goja.Value, error) {
	return goja.Undefined(), errors.New(msg)
}

// Sleep waits the provided seconds before continuing the execution.
func (mi *K6) Sleep(secs float64) {
	ctx := mi.vu.Context()
	timer := time.NewTimer(time.Duration(secs * float64(time.Second)))
	select {
	case <-timer.C:
	case <-ctx.Done():
		timer.Stop()
	}
}

// RandomSeed sets the seed to the random generator used for this VU.
func (mi *K6) RandomSeed(seed int64) {
	randSource := rand.New(rand.NewSource(seed)).Float64 //nolint:gosec
	mi.vu.Runtime().SetRandSource(randSource)
}

// Group wraps a function call and executes it within the provided group name.
func (mi *K6) Group(name string, fn goja.Callable) (goja.Value, error) {
	state := mi.vu.State()
	if state == nil {
		return nil, ErrGroupInInitContext
	}

	if fn == nil {
		return nil, errors.New("group() requires a callback as a second argument")
	}

	g, err := state.Group.Group(name)
	if err != nil {
		return goja.Undefined(), err
	}

	old := state.Group
	state.Group = g

	shouldUpdateTag := state.Options.SystemTags.Has(metrics.TagGroup)
	if shouldUpdateTag {
		state.Tags.Set("group", g.Path)
	}
	defer func() {
		state.Group = old
		if shouldUpdateTag {
			state.Tags.Set("group", old.Path)
		}
	}()

	startTime := time.Now()
	ret, err := fn(goja.Undefined())
	t := time.Now()

	tags := state.CloneTags()

	ctx := mi.vu.Context()
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time:   t,
		Metric: state.BuiltinMetrics.GroupDuration,
		Tags:   metrics.IntoSampleTags(&tags),
		Value:  metrics.D(t.Sub(startTime)),
	})

	return ret, err
}

// Check will emit check metrics for the provided checks.
//nolint:cyclop
func (mi *K6) Check(arg0, checks goja.Value, extras ...goja.Value) (bool, error) {
	state := mi.vu.State()
	if state == nil {
		return false, ErrCheckInInitContext
	}
	if checks == nil {
		return false, errors.New("no checks provided to `check`")
	}
	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	t := time.Now()

	// Prepare the metric tags
	commonTags := state.CloneTags()
	if len(extras) > 0 {
		obj := extras[0].ToObject(rt)
		for _, k := range obj.Keys() {
			commonTags[k] = obj.Get(k).String()
		}
	}

	succ := true
	var exc error
	obj := checks.ToObject(rt)
	for _, name := range obj.Keys() {
		val := obj.Get(name)

		tags := make(map[string]string, len(commonTags))
		for k, v := range commonTags {
			tags[k] = v
		}

		// Resolve the check record.
		check, err := state.Group.Check(name)
		if err != nil {
			return false, err
		}
		if state.Options.SystemTags.Has(metrics.TagCheck) {
			tags["check"] = check.Name
		}

		// Resolve callables into values.
		fn, ok := goja.AssertFunction(val)
		if ok {
			tmpVal, err := fn(goja.Undefined(), arg0)
			val = tmpVal
			if err != nil {
				val = rt.ToValue(false)
				exc = err
			}
		}

		sampleTags := metrics.IntoSampleTags(&tags)

		// Emit! (But only if we have a valid context.)
		select {
		case <-ctx.Done():
		default:
			if val.ToBoolean() {
				atomic.AddInt64(&check.Passes, 1)
				metrics.PushIfNotDone(ctx, state.Samples,
					metrics.Sample{Time: t, Metric: state.BuiltinMetrics.Checks, Tags: sampleTags, Value: 1})
			} else {
				atomic.AddInt64(&check.Fails, 1)
				metrics.PushIfNotDone(ctx, state.Samples,
					metrics.Sample{Time: t, Metric: state.BuiltinMetrics.Checks, Tags: sampleTags, Value: 0})
				// A single failure makes the return value false.
				succ = false
			}
		}

		if exc != nil {
			return succ, exc
		}
	}

	return succ, nil
}
