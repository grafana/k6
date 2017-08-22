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

package core

import (
	"context"
	"testing"
	"time"

	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

type testErrorWithString string

func (e testErrorWithString) Error() string  { return string(e) }
func (e testErrorWithString) String() string { return string(e) }

// Apply a null logger to the engine and return the hook.
func applyNullLogger(e *Engine) *logtest.Hook {
	logger, hook := logtest.NewNullLogger()
	e.SetLogger(logger)
	return hook
}

// Wrapper around newEngine that applies a null logger.
func newTestEngine(ex lib.Executor, opts lib.Options) (*Engine, error, *logtest.Hook) {
	e, err := NewEngine(ex, opts)
	if err != nil {
		return e, err, nil
	}
	hook := applyNullLogger(e)
	return e, nil, hook
}

func L(r lib.Runner) lib.Executor {
	return local.New(r)
}

func LF(fn func(ctx context.Context) ([]stats.Sample, error)) lib.Executor {
	return L(lib.RunnerFunc(fn))
}

func TestNewEngine(t *testing.T) {
	_, err, _ := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)
}

func TestNewEngineOptions(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Duration: lib.NullDurationFrom(10 * time.Second),
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[0], lib.Stage{Duration: lib.NullDurationFrom(10 * time.Second)})
		}
		assert.Equal(t, lib.NullDurationFrom(10*time.Second), e.Executor.GetEndTime())

		t.Run("Infinite", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{Duration: lib.NullDurationFrom(0)})
			assert.NoError(t, err)
			assert.Equal(t, []lib.Stage{{}}, e.Stages)
			assert.Equal(t, lib.NullDuration{}, e.Executor.GetEndTime())
		})
	})
	t.Run("Stages", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Stages: []lib.Stage{
				{Duration: lib.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[0], lib.Stage{Duration: lib.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)})
		}
		assert.Equal(t, lib.NullDurationFrom(10*time.Second), e.Executor.GetEndTime())
	})
	t.Run("Stages/Duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Duration: lib.NullDurationFrom(60 * time.Second),
			Stages: []lib.Stage{
				{Duration: lib.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[0], lib.Stage{Duration: lib.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)})
		}
		assert.Equal(t, lib.NullDurationFrom(10*time.Second), e.Executor.GetEndTime())
	})
	t.Run("Iterations", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{Iterations: null.IntFrom(100)})
		assert.NoError(t, err)
		assert.Equal(t, null.IntFrom(100), e.Executor.GetEndIterations())
	})
	t.Run("VUsMax", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.Equal(t, int64(0), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
		t.Run("set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(0), e.Executor.GetVUs())
		})
	})
	t.Run("VUs", func(t *testing.T) {
		t.Run("no max", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, lib.Options{
				VUs: null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (0)")
		})
		t.Run("max too low", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(1),
				VUs:    null.IntFrom(10),
			})
			assert.EqualError(t, err, "can't raise vu count (to 10) above vu cap (1)")
		})
		t.Run("max higher", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(1),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(1), e.Executor.GetVUs())
		})
		t.Run("max just right", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.Executor.GetVUsMax())
			assert.Equal(t, int64(10), e.Executor.GetVUs())
		})
	})
	t.Run("Paused", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("false", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(false),
			})
			assert.NoError(t, err)
			assert.False(t, e.Executor.IsPaused())
		})
		t.Run("true", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				Paused: null.BoolFrom(true),
			})
			assert.NoError(t, err)
			assert.True(t, e.Executor.IsPaused())
		})
	})
	t.Run("thresholds", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric": {},
			},
		})
		assert.NoError(t, err)
		assert.Contains(t, e.thresholds, "my_metric")

		t.Run("submetrics", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, lib.Options{
				Thresholds: map[string]stats.Thresholds{
					"my_metric{tag:value}": {},
				},
			})
			assert.NoError(t, err)
			assert.Contains(t, e.thresholds, "my_metric{tag:value}")
			assert.Contains(t, e.submetrics, "my_metric")
		})
	})
}

func TestEngineRun(t *testing.T) {
	t.Run("exits with context", func(t *testing.T) {
		duration := 100 * time.Millisecond
		e, err, _ := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		startTime := time.Now()
		assert.NoError(t, e.Run(ctx))
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
	})
	t.Run("exits with iterations", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			VUs:        null.IntFrom(10),
			VUsMax:     null.IntFrom(10),
			Iterations: null.IntFrom(100),
		})
		assert.NoError(t, err)
		assert.NoError(t, e.Run(context.Background()))
		assert.Equal(t, int64(100), e.Executor.GetIterations())
	})
	t.Run("exits with duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{
			VUs:      null.IntFrom(10),
			VUsMax:   null.IntFrom(10),
			Duration: lib.NullDurationFrom(1 * time.Second),
		})
		assert.NoError(t, err)
		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background()))
		assert.True(t, time.Now().After(startTime.Add(1*time.Second)))
	})
	t.Run("exits with stages", func(t *testing.T) {
		testdata := map[string]struct {
			Duration time.Duration
			Stages   []lib.Stage
		}{
			"none": {},
			"one": {
				1 * time.Second,
				[]lib.Stage{{Duration: lib.NullDurationFrom(1 * time.Second)}},
			},
			"two": {
				2 * time.Second,
				[]lib.Stage{
					{Duration: lib.NullDurationFrom(1 * time.Second)},
					{Duration: lib.NullDurationFrom(1 * time.Second)},
				},
			},
			"two/targeted": {
				2 * time.Second,
				[]lib.Stage{
					{Duration: lib.NullDurationFrom(1 * time.Second), Target: null.IntFrom(5)},
					{Duration: lib.NullDurationFrom(1 * time.Second), Target: null.IntFrom(10)},
				},
			},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				e, err, _ := newTestEngine(nil, lib.Options{
					VUs:    null.IntFrom(10),
					VUsMax: null.IntFrom(10),
				})
				assert.NoError(t, err)

				e.Stages = data.Stages
				startTime := time.Now()
				assert.NoError(t, e.Run(context.Background()))
				assert.WithinDuration(t,
					startTime.Add(data.Duration),
					startTime.Add(e.Executor.GetTime()),
					100*TickRate,
				)
			})
		}
	})
	t.Run("collects samples", func(t *testing.T) {
		testMetric := stats.New("test_metric", stats.Trend)

		signalChan := make(chan interface{})
		var e *Engine
		e, err, _ := newTestEngine(LF(func(ctx context.Context) (samples []stats.Sample, err error) {
			samples = append(samples, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 1})
			close(signalChan)
			<-ctx.Done()
			samples = append(samples, stats.Sample{Metric: testMetric, Time: time.Now(), Value: 2})
			return samples, err
		}), lib.Options{
			VUs:        null.IntFrom(1),
			VUsMax:     null.IntFrom(1),
			Iterations: null.IntFrom(1),
		})
		if !assert.NoError(t, err) {
			return
		}

		c := &dummy.Collector{}
		e.Collector = c

		ctx, cancel := context.WithCancel(context.Background())
		errC := make(chan error)
		go func() { errC <- e.Run(ctx) }()
		<-signalChan
		cancel()
		assert.NoError(t, <-errC)

		found := 0
		for _, s := range c.Samples {
			if s.Metric != testMetric {
				continue
			}
			found++
			assert.Equal(t, 1.0, s.Value, "wrong value")
		}
		assert.Equal(t, 1, found, "wrong number of samples")
	})
}

func TestEngineAtTime(t *testing.T) {
	e, err, _ := newTestEngine(nil, lib.Options{})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.NoError(t, e.Run(ctx))
}

func TestEngineCollector(t *testing.T) {
	testMetric := stats.New("test_metric", stats.Trend)

	e, err, _ := newTestEngine(LF(func(ctx context.Context) ([]stats.Sample, error) {
		return []stats.Sample{{Metric: testMetric}}, nil
	}), lib.Options{VUs: null.IntFrom(1), VUsMax: null.IntFrom(1), Iterations: null.IntFrom(1)})
	assert.NoError(t, err)

	c := &dummy.Collector{}
	e.Collector = c

	assert.NoError(t, e.Run(context.Background()))

	cSamples := []stats.Sample{}
	for _, sample := range c.Samples {
		if sample.Metric == testMetric {
			cSamples = append(cSamples, sample)
		}
	}
	metric := e.Metrics["test_metric"]
	if assert.NotNil(t, metric) {
		sink := metric.Sink.(*stats.TrendSink)
		if assert.NotNil(t, sink) {
			numCollectorSamples := len(cSamples)
			numEngineSamples := len(sink.Values)
			assert.Equal(t, numEngineSamples, numCollectorSamples)
		}
	}
}

func TestEngine_processSamples(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	t.Run("metric", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, lib.Options{})
		assert.NoError(t, err)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
	})
	t.Run("submetric", func(t *testing.T) {
		ths, err := stats.NewThresholds([]string{`1+1==2`})
		assert.NoError(t, err)

		e, err, _ := newTestEngine(nil, lib.Options{
			Thresholds: map[string]stats.Thresholds{
				"my_metric{a:1}": ths,
			},
		})
		assert.NoError(t, err)

		sms := e.submetrics["my_metric"]
		assert.Len(t, sms, 1)
		assert.Equal(t, "my_metric{a:1}", sms[0].Name)
		assert.EqualValues(t, map[string]string{"a": "1"}, sms[0].Tags)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric"].Sink)
		assert.IsType(t, &stats.GaugeSink{}, e.Metrics["my_metric{a:1}"].Sink)
	})
}

func TestEngine_processThresholds(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	testdata := map[string]struct {
		pass bool
		ths  map[string][]string
	}{
		"passing": {true, map[string][]string{"my_metric": {"1+1==2"}}},
		"failing": {false, map[string][]string{"my_metric": {"1+1==3"}}},

		"submetric,match,passing":   {true, map[string][]string{"my_metric{a:1}": {"1+1==2"}}},
		"submetric,match,failing":   {false, map[string][]string{"my_metric{a:1}": {"1+1==3"}}},
		"submetric,nomatch,passing": {true, map[string][]string{"my_metric{a:2}": {"1+1==2"}}},
		"submetric,nomatch,failing": {true, map[string][]string{"my_metric{a:2}": {"1+1==3"}}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			thresholds := make(map[string]stats.Thresholds, len(data.ths))
			for m, srcs := range data.ths {
				ths, err := stats.NewThresholds(srcs)
				assert.NoError(t, err)
				thresholds[m] = ths
			}

			e, err, _ := newTestEngine(nil, lib.Options{Thresholds: thresholds})
			assert.NoError(t, err)

			e.processSamples(
				stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
			)
			e.processThresholds()

			assert.Equal(t, data.pass, !e.IsTainted())
		})
	}
}
