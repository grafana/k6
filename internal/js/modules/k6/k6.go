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
		vu          modules.VU
		asyncGroups bool
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
	asyncGroups := asyncGroupsEnabled(vu)
	return &K6{vu: vu, asyncGroups: asyncGroups}
}

func asyncGroupsEnabled(vu modules.VU) bool {
	if state := vu.State(); state != nil {
		return common.AsyncMetricContextEnabled(state)
	}
	initEnv := vu.InitEnv()
	return initEnv != nil && initEnv.TestPreInitState != nil &&
		initEnv.FeatureFlags != nil && initEnv.FeatureFlags.AsyncMetricContext
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

// Group executes a callback under the provided group name.
//
// With the async-metric-context feature enabled, async callbacks retain their metric
// context through promise reactions and group_duration covers their lifetime.
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
	if !mi.asyncGroups && common.IsAsyncFunction(mi.vu.Runtime(), val) {
		return sobek.Undefined(), errors.New("group() does not support async functions as arguments, " +
			"please see https://grafana.com/docs/k6/latest/javascript-api/k6/group/ for more info")
	}

	parentTagsAndMeta := state.Tags.GetCurrentValues()
	oldGroupName, _ := parentTagsAndMeta.Tags.Get(metrics.TagGroup.String())
	newGroupName, err := lib.NewGroupPath(oldGroupName, name)
	if err != nil {
		return sobek.Undefined(), err
	}

	shouldUpdateTag := state.Options.SystemTags.Has(metrics.TagGroup)
	if shouldUpdateTag {
		setGroupTag(state, newGroupName)
	}
	var groupTagsAndMeta metrics.TagsAndMeta
	if mi.asyncGroups {
		groupTagsAndMeta = state.Tags.GetCurrentValues()
	}
	synchronous := true
	var startTime, endTime time.Time
	defer func() {
		if synchronous {
			if endTime.IsZero() {
				endTime = time.Now()
			}
			durationTagsAndMeta := groupTagsAndMeta
			if !mi.asyncGroups {
				durationTagsAndMeta = state.Tags.GetCurrentValues()
			}
			emitGroupDuration(mi.vu, startTime, endTime, durationTagsAndMeta)
		}
		if mi.asyncGroups {
			state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				*tagsAndMeta = parentTagsAndMeta
			})
		} else if shouldUpdateTag {
			setGroupTag(state, oldGroupName)
		}
	}()

	startTime = time.Now()
	ret, err := fn(sobek.Undefined())
	endTime = time.Now()
	if err != nil || !mi.asyncGroups {
		return ret, err
	}

	thenFn, isThenable := asThenable(ret)
	if !isThenable {
		return ret, nil
	}

	asyncResult, settled, err := mi.runAsyncGroup(startTime, groupTagsAndMeta, thenFn, ret)
	if err != nil {
		// A custom thenable may invoke a settlement handler synchronously. In that case the handler
		// emitted the duration before its rejection propagated back through then().
		synchronous = !settled
		return nil, err
	}
	// Settlement handlers now own duration emission. Until they are installed successfully, the
	// deferred synchronous path must retain ownership so an initialization error still emits once.
	synchronous = false
	return asyncResult, nil
}

func setGroupTag(state *lib.State, name string) {
	state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, name)
	})
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
