/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package local

import (
	"context"
	"errors"
	"net"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/executor"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func newTestExecutionScheduler(
	t *testing.T, runner lib.Runner, logger *logrus.Logger, opts lib.Options, //nolint: golint
) (ctx context.Context, cancel func(), execScheduler *ExecutionScheduler, samples chan stats.SampleContainer) {
	if runner == nil {
		runner = &lib.MiniRunner{}
	}
	ctx, cancel = context.WithCancel(context.Background())
	newOpts, err := executor.DeriveExecutionFromShortcuts(lib.Options{
		MetricSamplesBufferSize: null.NewInt(200, false),
	}.Apply(runner.GetOptions()).Apply(opts))
	require.NoError(t, err)
	require.Empty(t, newOpts.Validate())

	require.NoError(t, runner.SetOptions(newOpts))

	if logger == nil {
		logger = logrus.New()
		logger.SetOutput(testutils.NewTestOutput(t))
	}

	execScheduler, err = NewExecutionScheduler(runner, logger)
	require.NoError(t, err)

	samples = make(chan stats.SampleContainer, newOpts.MetricSamplesBufferSize.Int64)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	require.NoError(t, execScheduler.Init(ctx, samples))

	return ctx, cancel, execScheduler, samples
}

func TestExecutionSchedulerRun(t *testing.T) {
	t.Parallel()
	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, nil, nil, lib.Options{})
	defer cancel()

	err := make(chan error, 1)
	go func() { err <- execScheduler.Run(ctx, samples) }()
	assert.NoError(t, <-err)
}

func TestExecutionSchedulerSetupTeardownRun(t *testing.T) {
	t.Parallel()
	t.Run("Normal", func(t *testing.T) {
		setupC := make(chan struct{})
		teardownC := make(chan struct{})
		runner := &lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				close(setupC)
				return nil, nil
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				close(teardownC)
				return nil
			},
		}
		ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{})

		err := make(chan error, 1)
		go func() { err <- execScheduler.Run(ctx, samples) }()
		defer cancel()
		<-setupC
		<-teardownC
		assert.NoError(t, <-err)
	})
	t.Run("Setup Error", func(t *testing.T) {
		runner := &lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, errors.New("setup error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{})
		defer cancel()
		assert.EqualError(t, execScheduler.Run(ctx, samples), "setup error")
	})
	t.Run("Don't Run Setup", func(t *testing.T) {
		runner := &lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, errors.New("setup error")
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{
			NoSetup:    null.BoolFrom(true),
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()
		assert.EqualError(t, execScheduler.Run(ctx, samples), "teardown error")
	})

	t.Run("Teardown Error", func(t *testing.T) {
		runner := &lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, nil
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()

		assert.EqualError(t, execScheduler.Run(ctx, samples), "teardown error")
	})
	t.Run("Don't Run Teardown", func(t *testing.T) {
		runner := &lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, nil
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				return errors.New("teardown error")
			},
		}
		ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{
			NoTeardown: null.BoolFrom(true),
			VUs:        null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		defer cancel()
		assert.NoError(t, execScheduler.Run(ctx, samples))
	})
}

func TestExecutionSchedulerStages(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		Duration time.Duration
		Stages   []lib.Stage
	}{
		"one": {
			1 * time.Second,
			[]lib.Stage{{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(1)}},
		},
		"two": {
			2 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(1)},
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(2)},
			},
		},
		"four": {
			4 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(5)},
				{Duration: types.NullDurationFrom(3 * time.Second), Target: null.IntFrom(10)},
			},
		},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner := &lib.MiniRunner{
				Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
					time.Sleep(100 * time.Millisecond)
					return nil
				},
			}
			ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{
				VUs:    null.IntFrom(1),
				Stages: data.Stages,
			})
			defer cancel()
			assert.NoError(t, execScheduler.Run(ctx, samples))
			assert.True(t, execScheduler.GetState().GetCurrentTestRunDuration() >= data.Duration)
		})
	}
}

func TestExecutionSchedulerEndTime(t *testing.T) {
	t.Parallel()
	runner := &lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}
	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, nil, lib.Options{
		VUs:      null.IntFrom(10),
		Duration: types.NullDurationFrom(1 * time.Second),
	})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 31*time.Second, endTime) // because of the default 30s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	assert.NoError(t, execScheduler.Run(ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")
}

func TestExecutionSchedulerRuntimeErrors(t *testing.T) {
	t.Parallel()
	runner := &lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			time.Sleep(10 * time.Millisecond)
			return errors.New("hi")
		},
		Options: lib.Options{
			VUs:      null.IntFrom(10),
			Duration: types.NullDurationFrom(1 * time.Second),
		},
	}
	logger, hook := logtest.NewNullLogger()
	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 31*time.Second, endTime) // because of the default 30s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	assert.NoError(t, execScheduler.Run(ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")

	assert.NotEmpty(t, hook.Entries)
	for _, e := range hook.Entries {
		assert.Equal(t, "hi", e.Message)
	}
}

func TestExecutionSchedulerEndErrors(t *testing.T) {
	t.Parallel()

	exec := executor.NewConstantLoopingVUsConfig("we_need_hard_stop")
	exec.VUs = null.IntFrom(10)
	exec.Duration = types.NullDurationFrom(1 * time.Second)
	exec.GracefulStop = types.NullDurationFrom(0 * time.Second)

	runner := &lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			<-ctx.Done()
			return errors.New("hi")
		},
		Options: lib.Options{
			Execution: lib.ExecutorConfigMap{exec.GetName(): exec},
		},
	}
	logger, hook := logtest.NewNullLogger()
	ctx, cancel, execScheduler, samples := newTestExecutionScheduler(t, runner, logger, lib.Options{})
	defer cancel()

	endTime, isFinal := lib.GetEndOffset(execScheduler.GetExecutionPlan())
	assert.Equal(t, 1*time.Second, endTime) // because of the 0s gracefulStop
	assert.True(t, isFinal)

	startTime := time.Now()
	assert.NoError(t, execScheduler.Run(ctx, samples))
	runTime := time.Since(startTime)
	assert.True(t, runTime > 1*time.Second, "test did not take 1s")
	assert.True(t, runTime < 10*time.Second, "took more than 10 seconds")

	assert.Empty(t, hook.Entries)
}

func TestExecutionSchedulerEndIterations(t *testing.T) {
	t.Parallel()
	metric := &stats.Metric{Name: "test_metric"}

	options, err := executor.DeriveExecutionFromShortcuts(lib.Options{
		VUs:        null.IntFrom(1),
		Iterations: null.IntFrom(100),
	})
	require.NoError(t, err)
	require.Empty(t, options.Validate())

	var i int64
	runner := &lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			select {
			case <-ctx.Done():
			default:
				atomic.AddInt64(&i, 1)
			}
			out <- stats.Sample{Metric: metric, Value: 1.0}
			return nil
		},
		Options: options,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))

	execScheduler, err := NewExecutionScheduler(runner, logger)
	require.NoError(t, err)

	samples := make(chan stats.SampleContainer, 300)
	require.NoError(t, execScheduler.Init(ctx, samples))
	require.NoError(t, execScheduler.Run(ctx, samples))

	assert.Equal(t, uint64(100), execScheduler.GetState().GetFullIterationCount())
	assert.Equal(t, uint64(0), execScheduler.GetState().GetPartialIterationCount())
	assert.Equal(t, int64(100), i)
	require.Equal(t, 100, len(samples)) //TODO: change to 200 https://github.com/loadimpact/k6/issues/1250
	for i := 0; i < 100; i++ {
		mySample, ok := <-samples
		require.True(t, ok)
		assert.Equal(t, stats.Sample{Metric: metric, Value: 1.0}, mySample)
	}
}

func TestExecutionSchedulerIsRunning(t *testing.T) {
	t.Parallel()
	runner := &lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			<-ctx.Done()
			return nil
		},
	}
	ctx, cancel, execScheduler, _ := newTestExecutionScheduler(t, runner, nil, lib.Options{})
	state := execScheduler.GetState()

	err := make(chan error)
	go func() { err <- execScheduler.Run(ctx, nil) }()
	for !state.HasStarted() {
		time.Sleep(10 * time.Microsecond)
	}
	cancel()
	for !state.HasEnded() {
		time.Sleep(10 * time.Microsecond)
	}
	assert.NoError(t, <-err)
}

/*
//TODO: convert for the externally-controlled scheduler
func TestExecutionSchedulerSetVUs(t *testing.T) {
	t.Run("Negative", func(t *testing.T) {
		assert.EqualError(t, New(nil).SetVUs(-1), "vu count can't be negative")
	})

	t.Run("Too High", func(t *testing.T) {
		assert.EqualError(t, New(nil).SetVUs(100), "can't raise vu count (to 100) above vu cap (0)")
	})

	t.Run("Raise", func(t *testing.T) {
		e := New(&lib.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			return nil
		}})
		e.ctx = context.Background()

		assert.NoError(t, e.SetVUsMax(100))
		assert.Equal(t, int64(100), e.GetVUsMax())
		if assert.Len(t, e.vus, 100) {
			num := 0
			for i, handle := range e.vus {
				num++
				if assert.NotNil(t, handle.vu, "vu %d lacks impl", i) {
					assert.Equal(t, int64(0), handle.vu.(*lib.MiniRunnerVU).ID)
				}
				assert.Nil(t, handle.ctx, "vu %d has ctx", i)
				assert.Nil(t, handle.cancel, "vu %d has cancel", i)
			}
			assert.Equal(t, 100, num)
		}

		assert.NoError(t, e.SetVUs(50))
		assert.Equal(t, int64(50), e.GetVUs())
		if assert.Len(t, e.vus, 100) {
			num := 0
			for i, handle := range e.vus {
				if i < 50 {
					assert.NotNil(t, handle.cancel, "vu %d lacks cancel", i)
					assert.Equal(t, int64(i+1), handle.vu.(*lib.MiniRunnerVU).ID)
					num++
				} else {
					assert.Nil(t, handle.cancel, "vu %d has cancel", i)
					assert.Equal(t, int64(0), handle.vu.(*lib.MiniRunnerVU).ID)
				}
			}
			assert.Equal(t, 50, num)
		}

		assert.NoError(t, e.SetVUs(100))
		assert.Equal(t, int64(100), e.GetVUs())
		if assert.Len(t, e.vus, 100) {
			num := 0
			for i, handle := range e.vus {
				assert.NotNil(t, handle.cancel, "vu %d lacks cancel", i)
				assert.Equal(t, int64(i+1), handle.vu.(*lib.MiniRunnerVU).ID)
				num++
			}
			assert.Equal(t, 100, num)
		}

		t.Run("Lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(50))
			assert.Equal(t, int64(50), e.GetVUs())
			if assert.Len(t, e.vus, 100) {
				num := 0
				for i, handle := range e.vus {
					if i < 50 {
						assert.NotNil(t, handle.cancel, "vu %d lacks cancel", i)
						num++
					} else {
						assert.Nil(t, handle.cancel, "vu %d has cancel", i)
					}
					assert.Equal(t, int64(i+1), handle.vu.(*lib.MiniRunnerVU).ID)
				}
				assert.Equal(t, 50, num)
			}

			t.Run("Raise", func(t *testing.T) {
				assert.NoError(t, e.SetVUs(100))
				assert.Equal(t, int64(100), e.GetVUs())
				if assert.Len(t, e.vus, 100) {
					for i, handle := range e.vus {
						assert.NotNil(t, handle.cancel, "vu %d lacks cancel", i)
						if i < 50 {
							assert.Equal(t, int64(i+1), handle.vu.(*lib.MiniRunnerVU).ID)
						} else {
							assert.Equal(t, int64(50+i+1), handle.vu.(*lib.MiniRunnerVU).ID)
						}
					}
				}
			})
		})
	})
}
*/

func TestRealTimeAndSetupTeardownMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	t.Parallel()
	script := []byte(`
	import { Counter } from "k6/metrics";
	import { sleep } from "k6";

	var counter = new Counter("test_counter");

	export function setup() {
		console.log("setup(), sleeping for 1 second");
		counter.add(1, { place: "setupBeforeSleep" });
		sleep(1);
		console.log("setup sleep is done");
		counter.add(2, { place: "setupAfterSleep" });
		return { "some": ["data"], "v": 1 };
	}

	export function teardown(data) {
		console.log("teardown(" + JSON.stringify(data) + "), sleeping for 1 second");
		counter.add(3, { place: "teardownBeforeSleep" });
		sleep(1);
		if (!data || data.v != 1) {
			throw new Error("incorrect data: " + JSON.stringify(data));
		}
		console.log("teardown sleep is done");
		counter.add(4, { place: "teardownAfterSleep" });
	}

	export default function (data) {
		console.log("default(" + JSON.stringify(data) + ") with ENV=" + JSON.stringify(__ENV) + " for in ITER " + __ITER + " and VU " + __VU);
		counter.add(5, { place: "defaultBeforeSleep" });
		if (!data || data.v != 1) {
			throw new Error("incorrect data: " + JSON.stringify(data));
		}
		sleep(1);
		console.log("default() for in ITER " + __ITER + " and VU " + __VU + " done!");
		counter.add(6, { place: "defaultAfterSleep" });
	}`)

	runner, err := js.New(&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script}, nil, lib.RuntimeOptions{})
	require.NoError(t, err)

	options, err := executor.DeriveExecutionFromShortcuts(runner.GetOptions().Apply(lib.Options{
		Iterations:      null.IntFrom(2),
		VUs:             null.IntFrom(1),
		SystemTags:      &stats.DefaultSystemTagSet,
		SetupTimeout:    types.NullDurationFrom(4 * time.Second),
		TeardownTimeout: types.NullDurationFrom(4 * time.Second),
	}))
	require.NoError(t, err)
	require.NoError(t, runner.SetOptions(options))

	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))

	execScheduler, err := NewExecutionScheduler(runner, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	sampleContainers := make(chan stats.SampleContainer)
	go func() {
		require.NoError(t, execScheduler.Init(ctx, sampleContainers))
		assert.NoError(t, execScheduler.Run(ctx, sampleContainers))
		close(done)
	}()

	expectIn := func(from, to time.Duration, expected stats.SampleContainer) {
		start := time.Now()
		from = from * time.Millisecond
		to = to * time.Millisecond
		for {
			select {
			case sampleContainer := <-sampleContainers:
				now := time.Now()
				elapsed := now.Sub(start)
				if elapsed < from {
					t.Errorf("Received sample earlier (%s) than expected (%s)", elapsed, from)
					return
				}
				assert.IsType(t, expected, sampleContainer)
				expSamples := expected.GetSamples()
				gotSamples := sampleContainer.GetSamples()
				if assert.Len(t, gotSamples, len(expSamples)) {
					for i, s := range gotSamples {
						expS := expSamples[i]
						if s.Metric != metrics.IterationDuration {
							assert.Equal(t, expS.Value, s.Value)
						}
						assert.Equal(t, expS.Metric.Name, s.Metric.Name)
						assert.Equal(t, expS.Tags.CloneTags(), s.Tags.CloneTags())
						assert.InDelta(t, 0, now.Sub(s.Time), float64(50*time.Millisecond))
					}
				}
				return
			case <-time.After(to):
				t.Errorf("Did not receive sample in the maximum allotted time (%s)", to)
				return
			}
		}
	}

	getTags := func(args ...string) *stats.SampleTags {
		tags := map[string]string{}
		for i := 0; i < len(args)-1; i += 2 {
			tags[args[i]] = args[i+1]
		}
		return stats.IntoSampleTags(&tags)
	}
	testCounter := stats.New("test_counter", stats.Counter)
	getSample := func(expValue float64, expMetric *stats.Metric, expTags ...string) stats.SampleContainer {
		return stats.Sample{
			Metric: expMetric,
			Time:   time.Now(),
			Tags:   getTags(expTags...),
			Value:  expValue,
		}
	}
	getDummyTrail := func(group string, emitIterations bool) stats.SampleContainer {
		return netext.NewDialer(net.Dialer{}).GetTrail(time.Now(), time.Now(),
			true, emitIterations, getTags("group", group))
	}

	// Initially give a long time (5s) for the execScheduler to start
	expectIn(0, 5000, getSample(1, testCounter, "group", "::setup", "place", "setupBeforeSleep"))
	expectIn(900, 1100, getSample(2, testCounter, "group", "::setup", "place", "setupAfterSleep"))
	expectIn(0, 100, getDummyTrail("::setup", false))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep"))
	expectIn(0, 100, getDummyTrail("", true))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep"))
	expectIn(0, 100, getDummyTrail("", true))

	expectIn(0, 1000, getSample(3, testCounter, "group", "::teardown", "place", "teardownBeforeSleep"))
	expectIn(900, 1100, getSample(4, testCounter, "group", "::teardown", "place", "teardownAfterSleep"))
	expectIn(0, 100, getDummyTrail("::teardown", false))

	for {
		select {
		case s := <-sampleContainers:
			t.Fatalf("Did not expect anything in the sample channel bug got %#v", s)
		case <-time.After(3 * time.Second):
			t.Fatalf("Local execScheduler took way to long to finish")
		case <-done:
			return // Exit normally
		}
	}
}
