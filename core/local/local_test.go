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
	"net"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/loader"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func TestExecutorRun(t *testing.T) {
	e := New(nil)
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))

	ctx, cancel := context.WithCancel(context.Background())
	err := make(chan error, 1)
	samples := make(chan stats.SampleContainer, 100)
	defer close(samples)
	go func() {
		for range samples {
		}
	}()

	go func() { err <- e.Run(ctx, samples) }()
	cancel()
	assert.NoError(t, <-err)
}

func TestExecutorSetupTeardownRun(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		setupC := make(chan struct{})
		teardownC := make(chan struct{})
		e := New(&lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				close(setupC)
				return nil, nil
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				close(teardownC)
				return nil
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		err := make(chan error, 1)
		go func() { err <- e.Run(ctx, make(chan stats.SampleContainer, 100)) }()
		cancel()
		<-setupC
		<-teardownC
		assert.NoError(t, <-err)
	})
	t.Run("Setup Error", func(t *testing.T) {
		e := New(&lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, errors.New("setup error")
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				return errors.New("teardown error")
			},
		})
		assert.EqualError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 100)), "setup error")

		t.Run("Don't Run Setup", func(t *testing.T) {
			e := New(&lib.MiniRunner{
				SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
					return nil, errors.New("setup error")
				},
				TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
					return errors.New("teardown error")
				},
			})
			e.SetRunSetup(false)
			e.SetEndIterations(null.IntFrom(1))
			assert.NoError(t, e.SetVUsMax(1))
			assert.NoError(t, e.SetVUs(1))
			assert.EqualError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 100)), "teardown error")
		})
	})
	t.Run("Teardown Error", func(t *testing.T) {
		e := New(&lib.MiniRunner{
			SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
				return nil, nil
			},
			TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				return errors.New("teardown error")
			},
		})
		e.SetEndIterations(null.IntFrom(1))
		assert.NoError(t, e.SetVUsMax(1))
		assert.NoError(t, e.SetVUs(1))
		assert.EqualError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 100)), "teardown error")

		t.Run("Don't Run Teardown", func(t *testing.T) {
			e := New(&lib.MiniRunner{
				SetupFn: func(ctx context.Context, out chan<- stats.SampleContainer) ([]byte, error) {
					return nil, nil
				},
				TeardownFn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
					return errors.New("teardown error")
				},
			})
			e.SetRunTeardown(false)
			e.SetEndIterations(null.IntFrom(1))
			assert.NoError(t, e.SetVUsMax(1))
			assert.NoError(t, e.SetVUs(1))
			assert.NoError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 100)))
		})
	})
}

func TestExecutorSetLogger(t *testing.T) {
	logger, _ := logtest.NewNullLogger()
	e := New(nil)
	e.SetLogger(logger)
	assert.Equal(t, logger, e.GetLogger())
}

func TestExecutorStages(t *testing.T) {
	testdata := map[string]struct {
		Duration time.Duration
		Stages   []lib.Stage
	}{
		"one": {
			1 * time.Second,
			[]lib.Stage{{Duration: types.NullDurationFrom(1 * time.Second)}},
		},
		"two": {
			2 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second)},
				{Duration: types.NullDurationFrom(1 * time.Second)},
			},
		},
		"two/targeted": {
			2 * time.Second,
			[]lib.Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(5)},
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(10)},
			},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			e := New(&lib.MiniRunner{
				Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
					time.Sleep(100 * time.Millisecond)
					return nil
				},
				Options: lib.Options{
					MetricSamplesBufferSize: null.IntFrom(500),
				},
			})
			assert.NoError(t, e.SetVUsMax(10))
			e.SetStages(data.Stages)
			assert.NoError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 500)))
			assert.True(t, e.GetTime() >= data.Duration)
		})
	}
}

func TestExecutorEndTime(t *testing.T) {
	e := New(&lib.MiniRunner{
		Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
		Options: lib.Options{MetricSamplesBufferSize: null.IntFrom(200)},
	})
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))
	e.SetEndTime(types.NullDurationFrom(1 * time.Second))
	assert.Equal(t, types.NullDurationFrom(1*time.Second), e.GetEndTime())

	startTime := time.Now()
	assert.NoError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 200)))
	assert.True(t, time.Now().After(startTime.Add(1*time.Second)), "test did not take 1s")

	t.Run("Runtime Errors", func(t *testing.T) {
		e := New(&lib.MiniRunner{
			Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				time.Sleep(10 * time.Millisecond)
				return errors.New("hi")
			},
			Options: lib.Options{MetricSamplesBufferSize: null.IntFrom(200)},
		})
		assert.NoError(t, e.SetVUsMax(10))
		assert.NoError(t, e.SetVUs(10))
		e.SetEndTime(types.NullDurationFrom(100 * time.Millisecond))
		assert.Equal(t, types.NullDurationFrom(100*time.Millisecond), e.GetEndTime())

		l, hook := logtest.NewNullLogger()
		e.SetLogger(l)

		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 200)))
		assert.True(t, time.Now().After(startTime.Add(100*time.Millisecond)), "test did not take 100ms")

		assert.NotEmpty(t, hook.Entries)
		for _, e := range hook.Entries {
			assert.Equal(t, "hi", e.Message)
		}
	})

	t.Run("End Errors", func(t *testing.T) {
		e := New(&lib.MiniRunner{
			Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
				<-ctx.Done()
				return errors.New("hi")
			},
			Options: lib.Options{MetricSamplesBufferSize: null.IntFrom(200)},
		})
		assert.NoError(t, e.SetVUsMax(10))
		assert.NoError(t, e.SetVUs(10))
		e.SetEndTime(types.NullDurationFrom(100 * time.Millisecond))
		assert.Equal(t, types.NullDurationFrom(100*time.Millisecond), e.GetEndTime())

		l, hook := logtest.NewNullLogger()
		e.SetLogger(l)

		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background(), make(chan stats.SampleContainer, 200)))
		assert.True(t, time.Now().After(startTime.Add(100*time.Millisecond)), "test did not take 100ms")

		assert.Empty(t, hook.Entries)
	})
}

func TestExecutorEndIterations(t *testing.T) {
	metric := &stats.Metric{Name: "test_metric"}

	var i int64
	e := New(&lib.MiniRunner{Fn: func(ctx context.Context, out chan<- stats.SampleContainer) error {
		select {
		case <-ctx.Done():
		default:
			atomic.AddInt64(&i, 1)
		}
		out <- stats.Sample{Metric: metric, Value: 1.0}
		return nil
	}})
	assert.NoError(t, e.SetVUsMax(1))
	assert.NoError(t, e.SetVUs(1))
	e.SetEndIterations(null.IntFrom(100))
	assert.Equal(t, null.IntFrom(100), e.GetEndIterations())

	samples := make(chan stats.SampleContainer, 201)
	assert.NoError(t, e.Run(context.Background(), samples))
	assert.Equal(t, int64(100), e.GetIterations())
	assert.Equal(t, int64(100), i)
	for i := 0; i < 100; i++ {
		mySample, ok := <-samples
		require.True(t, ok)
		assert.Equal(t, stats.Sample{Metric: metric, Value: 1.0}, mySample)
		sample, ok := <-samples
		require.True(t, ok)
		iterSample, ok := (sample).(stats.Sample)
		require.True(t, ok)
		assert.Equal(t, metrics.Iterations, iterSample.Metric)
		assert.Equal(t, float64(1), iterSample.Value)
	}
}

func TestExecutorIsRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e := New(nil)

	err := make(chan error)
	go func() { err <- e.Run(ctx, nil) }()
	for !e.IsRunning() {
	}
	cancel()
	for e.IsRunning() {
	}
	assert.NoError(t, <-err)
}

func TestExecutorSetVUsMax(t *testing.T) {
	t.Run("Negative", func(t *testing.T) {
		assert.EqualError(t, New(nil).SetVUsMax(-1), "vu cap can't be negative")
	})

	t.Run("Raise", func(t *testing.T) {
		e := New(nil)

		assert.NoError(t, e.SetVUsMax(50))
		assert.Equal(t, int64(50), e.GetVUsMax())

		assert.NoError(t, e.SetVUsMax(100))
		assert.Equal(t, int64(100), e.GetVUsMax())

		t.Run("Lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUsMax(50))
			assert.Equal(t, int64(50), e.GetVUsMax())
		})
	})

	t.Run("TooLow", func(t *testing.T) {
		e := New(nil)
		e.ctx = context.Background()

		assert.NoError(t, e.SetVUsMax(100))
		assert.Equal(t, int64(100), e.GetVUsMax())

		assert.NoError(t, e.SetVUs(100))
		assert.Equal(t, int64(100), e.GetVUs())

		assert.EqualError(t, e.SetVUsMax(50), "can't lower vu cap (to 50) below vu count (100)")
	})
}

func TestExecutorSetVUs(t *testing.T) {
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

	runner, err := js.New(
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	options := lib.Options{
		SystemTags:      stats.ToSystemTagSet(stats.DefaultSystemTagList),
		SetupTimeout:    types.NullDurationFrom(4 * time.Second),
		TeardownTimeout: types.NullDurationFrom(4 * time.Second),
	}
	runner.SetOptions(options)

	executor := New(runner)
	executor.SetEndIterations(null.IntFrom(2))
	require.NoError(t, executor.SetVUsMax(1))
	require.NoError(t, executor.SetVUs(1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	sampleContainers := make(chan stats.SampleContainer)
	go func() {
		assert.NoError(t, executor.Run(ctx, sampleContainers))
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
	getDummyTrail := func(group string) stats.SampleContainer {
		return netext.NewDialer(net.Dialer{}).GetTrail(time.Now(), time.Now(), true, getTags("group", group))
	}

	// Initially give a long time (5s) for the executor to start
	expectIn(0, 5000, getSample(1, testCounter, "group", "::setup", "place", "setupBeforeSleep"))
	expectIn(900, 1100, getSample(2, testCounter, "group", "::setup", "place", "setupAfterSleep"))
	expectIn(0, 100, getDummyTrail("::setup"))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep"))
	expectIn(0, 100, getDummyTrail(""))
	expectIn(0, 100, getSample(1, metrics.Iterations))

	expectIn(0, 100, getSample(5, testCounter, "group", "", "place", "defaultBeforeSleep"))
	expectIn(900, 1100, getSample(6, testCounter, "group", "", "place", "defaultAfterSleep"))
	expectIn(0, 100, getDummyTrail(""))
	expectIn(0, 100, getSample(1, metrics.Iterations))

	expectIn(0, 1000, getSample(3, testCounter, "group", "::teardown", "place", "teardownBeforeSleep"))
	expectIn(900, 1100, getSample(4, testCounter, "group", "::teardown", "place", "teardownAfterSleep"))
	expectIn(0, 100, getDummyTrail("::teardown"))

	for {
		select {
		case s := <-sampleContainers:
			t.Fatalf("Did not expect anything in the sample channel bug got %#v", s)
		case <-time.After(3 * time.Second):
			t.Fatalf("Local executor took way to long to finish")
		case <-done:
			return // Exit normally
		}
	}
}
