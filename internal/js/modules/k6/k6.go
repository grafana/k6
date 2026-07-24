// Package k6 implements the module imported as 'k6' from inside k6.
package k6

import (
	"errors"
	"math/rand" // nosemgrep: math-random-used // used to seed the Marh.random of the JS VM that is pseudo random by specification
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
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

		// groupSleep is the cumulative time this VU has spent in sleep() calls.
		// It is used to attribute sleep time to the group(s) currently being
		// executed: group() snapshots this value on entry and diffs it on exit,
		// so the difference is exactly the sleep that happened within that
		// group's window (correctly accounting for nested groups). A VU runs a
		// single iteration at a time on one goroutine and JS is single-threaded,
		// so this field needs no synchronization.
		groupSleep time.Duration
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
		Named: map[string]any{
			"check":      mi.Check,
			"fail":       mi.Fail,
			"group":      mi.Group,
			"randomSeed": mi.RandomSeed,
			"sleep":      mi.Sleep,
		},
	}
}

// Fail is a fancy way of saying `throw "something"`.
func (*K6) Fail(msg string) (sobek.Value, error) {
	return sobek.Undefined(), errors.New(msg)
}

// Sleep waits the provided seconds before continuing the execution.
func (mi *K6) Sleep(secs float64) {
	ctx := mi.vu.Context()
	start := time.Now()
	timer := time.NewTimer(time.Duration(secs * float64(time.Second)))
	select {
	case <-timer.C:
	case <-ctx.Done():
		timer.Stop()
	}
	// Account for the actual time spent sleeping (which may be shorter than
	// requested if the context was cancelled) so that group() can subtract it
	// from the enclosing group(s). See the K6.groupSleep field.
	mi.groupSleep += time.Since(start)
}

// RandomSeed sets the seed to the random generator used for this VU.
func (mi *K6) RandomSeed(seed int64) {
	randSource := rand.New(rand.NewSource(seed)).Float64 //nolint:gosec
	mi.vu.Runtime().SetRandSource(randSource)
}

// Group wraps a function call and executes it within the provided group name.
func (mi *K6) Group(name string, val sobek.Value) (sobek.Value, error) {
	state := mi.vu.State()
	if state == nil {
		return nil, ErrGroupInInitContext
	}

	if common.IsNullish(val) {
		return nil, errors.New("group() requires a callback as a second argument")
	}
	fn, ok := sobek.AssertFunction(val)
	if !ok {
		return nil, errors.New("group() requires a callback as a second argument")
	}
	if common.IsAsyncFunction(mi.vu.Runtime(), val) {
		return sobek.Undefined(), errors.New("group() does not support async functions as arguments, " +
			"please see https://grafana.com/docs/k6/latest/javascript-api/k6/group/ for more info")
	}
	oldGroupName, _ := state.Tags.GetCurrentValues().Tags.Get(metrics.TagGroup.String())
	// TODO: what are we doing if group is not tagged
	newGroupName, err := lib.NewGroupPath(oldGroupName, name)
	if err != nil {
		return sobek.Undefined(), err
	}

	shouldUpdateTag := state.Options.SystemTags.Has(metrics.TagGroup)
	if shouldUpdateTag {
		state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, newGroupName)
		})
	}
	defer func() {
		if shouldUpdateTag {
			state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, oldGroupName)
			})
		}
	}()

	startTime := time.Now()
	startSleep := mi.groupSleep
	ret, err := fn(sobek.Undefined())
	t := time.Now()
	// Sleep time that occurred within this group's execution window. For nested
	// groups this correctly captures only the sleep of the current window, since
	// each enclosing group takes its own snapshot/diff.
	groupSleep := mi.groupSleep - startSleep

	ctx := mi.vu.Context()
	ctm := state.Tags.GetCurrentValues()
	// group_duration keeps its historical wall-clock semantics (including sleep)
	// for backward compatibility, and is emitted as a standalone sample exactly
	// as before. group_sleep is emitted as a separate sample so users can measure
	// sleep per group and derive active time as group_duration - group_sleep.
	// See issue #921.
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GroupDuration,
			Tags:   ctm.Tags,
		},
		Time:     t,
		Value:    metrics.D(t.Sub(startTime)),
		Metadata: ctm.Metadata,
	})
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GroupSleep,
			Tags:   ctm.Tags,
		},
		Time:     t,
		Value:    metrics.D(groupSleep),
		Metadata: ctm.Metadata,
	})

	return ret, err
}

// Check will emit check metrics for the provided checks.
func (mi *K6) Check(arg0, checks sobek.Value, extras ...sobek.Value) (bool, error) {
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
	commonTagsAndMeta := state.Tags.GetCurrentValues()
	if len(extras) > 0 {
		if err := common.ApplyCustomUserTags(rt, &commonTagsAndMeta, extras[0]); err != nil {
			return false, err
		}
	}

	succ := true
	var exc error
	obj := checks.ToObject(rt)
	for _, name := range obj.Keys() {
		if strings.Contains(name, lib.GroupSeparator) {
			return false, lib.ErrNameContainsGroupSeparator
		}
		val := obj.Get(name)

		tags := commonTagsAndMeta.Tags
		if state.Options.SystemTags.Has(metrics.TagCheck) {
			tags = tags.With("check", name)
		}

		if common.IsAsyncFunction(rt, val) {
			return false, errors.New("the built-in check() does not support async functions as arguments. " +
				"Use the JavaScript utils library as a replacement. " +
				"Refer to https://grafana.com/docs/k6/latest/javascript-api/jslib/utils/check/ for more info")
		}

		// Resolve callables into values.
		fn, ok := sobek.AssertFunction(val)
		if ok {
			tmpVal, err := fn(sobek.Undefined(), arg0)
			val = tmpVal
			if err != nil {
				val = rt.ToValue(false)
				exc = err
			}
		}
		booleanVal := val.ToBoolean()
		if !booleanVal {
			// A single failure makes the return value false.
			succ = false
		}

		sample := metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: state.BuiltinMetrics.Checks,
				Tags:   tags,
			},
			Time:     t,
			Metadata: commonTagsAndMeta.Metadata,
		}
		if booleanVal {
			sample.Value = 1
		}

		metrics.PushIfNotDone(ctx, state.Samples, sample)

		if exc != nil {
			return false, exc
		}
	}

	return succ, nil
}
