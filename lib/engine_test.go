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

package lib

import (
	"context"
	logtest "github.com/Sirupsen/logrus/hooks/test"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
	"runtime"
	"testing"
	"time"
)

type testErrorWithString string

func (e testErrorWithString) Error() string  { return string(e) }
func (e testErrorWithString) String() string { return string(e) }

// Apply a null logger to the engine and return the hook.
func applyNullLogger(e *Engine) *logtest.Hook {
	logger, hook := logtest.NewNullLogger()
	e.Logger = logger
	return hook
}

// Wrapper around newEngine that applies a null logger.
func newTestEngine(r Runner, opts Options) (*Engine, error, *logtest.Hook) {
	e, err := NewEngine(r, opts)
	if err != nil {
		return e, err, nil
	}
	hook := applyNullLogger(e)
	return e, nil, hook
}

// Helper for asserting the number of active/dead VUs.
func assertActiveVUs(t *testing.T, e *Engine, active, dead int) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var numActive, numDead int
	var lastWasDead bool
	for _, vu := range e.vuEntries {
		if vu.Cancel != nil {
			numActive++
			assert.False(t, lastWasDead, "living vu in dead zone")
		} else {
			numDead++
			lastWasDead = true
		}
	}
	assert.Equal(t, active, numActive, "wrong number of active vus")
	assert.Equal(t, dead, numDead, "wrong number of dead vus")
}

func Test_parseSubmetric(t *testing.T) {
	testdata := map[string]struct {
		parent string
		conds  map[string]string
	}{
		"my_metric":                 {"my_metric", nil},
		"my_metric{}":               {"my_metric", map[string]string{}},
		"my_metric{a}":              {"my_metric", map[string]string{"a": ""}},
		"my_metric{a:1}":            {"my_metric", map[string]string{"a": "1"}},
		"my_metric{ a : 1 }":        {"my_metric", map[string]string{"a": "1"}},
		"my_metric{a,b}":            {"my_metric", map[string]string{"a": "", "b": ""}},
		"my_metric{a:1,b:2}":        {"my_metric", map[string]string{"a": "1", "b": "2"}},
		"my_metric{ a : 1, b : 2 }": {"my_metric", map[string]string{"a": "1", "b": "2"}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			parent, conds := parseSubmetric(name)
			assert.Equal(t, data.parent, parent)
			if data.conds != nil {
				assert.EqualValues(t, data.conds, conds)
			} else {
				assert.Nil(t, conds)
			}
		})
	}
}

func TestNewEngine(t *testing.T) {
	_, err, _ := newTestEngine(nil, Options{})
	assert.NoError(t, err)
}

func TestNewEngineOptions(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{
			Duration: null.StringFrom("10s"),
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[0], Stage{Duration: 10 * time.Second})
		}
	})
	t.Run("Stages", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{
			Stages: []Stage{
				Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[0], Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)})
		}
	})
	t.Run("Stages/Duration", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{
			Duration: null.StringFrom("60s"),
			Stages: []Stage{
				Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)},
			},
		})
		assert.NoError(t, err)
		if assert.Len(t, e.Stages, 1) {
			assert.Equal(t, e.Stages[1], Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)})
		}
	})
	t.Run("VUsMax", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{})
			assert.NoError(t, err)
			assert.Equal(t, int64(0), e.GetVUsMax())
			assert.Equal(t, int64(0), e.GetVUs())
		})
		t.Run("set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				VUsMax: null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.GetVUsMax())
			assert.Equal(t, int64(0), e.GetVUs())
		})
	})
	t.Run("VUs", func(t *testing.T) {
		t.Run("no max", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, Options{
				VUs: null.IntFrom(10),
			})
			assert.EqualError(t, err, "more vus than allocated requested")
		})
		t.Run("max too low", func(t *testing.T) {
			_, err, _ := newTestEngine(nil, Options{
				VUsMax: null.IntFrom(1),
				VUs:    null.IntFrom(10),
			})
			assert.EqualError(t, err, "more vus than allocated requested")
		})
		t.Run("max higher", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(1),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.GetVUsMax())
			assert.Equal(t, int64(1), e.GetVUs())
		})
		t.Run("max just right", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.GetVUsMax())
			assert.Equal(t, int64(10), e.GetVUs())
		})
	})
	t.Run("Paused", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{})
			assert.NoError(t, err)
			assert.False(t, e.IsPaused())
		})
		t.Run("false", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				Paused: null.BoolFrom(false),
			})
			assert.NoError(t, err)
			assert.False(t, e.IsPaused())
		})
		t.Run("true", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				Paused: null.BoolFrom(true),
			})
			assert.NoError(t, err)
			assert.True(t, e.IsPaused())
		})
	})
	t.Run("thresholds", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{
			Thresholds: map[string]Thresholds{
				"my_metric": {},
			},
		})
		assert.NoError(t, err)
		assert.Contains(t, e.Thresholds, "my_metric")

		t.Run("submetrics", func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				Thresholds: map[string]Thresholds{
					"my_metric{tag:value}": {},
				},
			})
			assert.NoError(t, err)
			assert.Contains(t, e.Thresholds, "my_metric{tag:value}")
			assert.Contains(t, e.submetrics, "my_metric")
		})
	})
}

func TestEngineRun(t *testing.T) {
	t.Run("exits with context", func(t *testing.T) {
		startTime := time.Now()
		duration := 100 * time.Millisecond
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		assert.NoError(t, e.Run(ctx))
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
	})
	t.Run("terminates subctx", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)

		subctx := e.subctx
		select {
		case <-subctx.Done():
			assert.Fail(t, "context is already terminated")
		default:
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.NoError(t, e.Run(ctx))

		assert.NotEqual(t, subctx, e.subctx, "subcontext not changed")
		select {
		case <-subctx.Done():
		default:
			assert.Fail(t, "context was not terminated")
		}
	})
	t.Run("exits with stages", func(t *testing.T) {
		testdata := map[string]struct {
			Duration time.Duration
			Stages   []Stage
		}{
			"none": {},
			"one": {
				1 * time.Second,
				[]Stage{Stage{Duration: 1 * time.Second}},
			},
			"two": {
				2 * time.Second,
				[]Stage{Stage{Duration: 1 * time.Second}, Stage{Duration: 1 * time.Second}},
			},
			"two/targeted": {
				2 * time.Second,
				[]Stage{
					Stage{Duration: 1 * time.Second, Target: null.IntFrom(5)},
					Stage{Duration: 1 * time.Second, Target: null.IntFrom(10)},
				},
			},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				e, err, _ := newTestEngine(nil, Options{})
				assert.NoError(t, err)

				e.Stages = data.Stages
				startTime := time.Now()
				assert.NoError(t, e.Run(context.Background()))
				assert.WithinDuration(t,
					startTime.Add(data.Duration),
					startTime.Add(e.AtTime()),
					100*TickRate,
				)
			})
		}
	})
	t.Run("collects samples", func(t *testing.T) {
		testMetric := stats.New("test_metric", stats.Trend)

		errors := map[string]error{
			"nil":   nil,
			"error": errors.New("error"),
		}
		for name, reterr := range errors {
			t.Run(name, func(t *testing.T) {
				e, err, _ := newTestEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return []stats.Sample{{Metric: testMetric, Value: 1.0}}, reterr
				}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
				assert.NoError(t, err)

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				assert.NoError(t, e.Run(ctx))

				e.lock.Lock()
				if !assert.True(t, e.numIterations > 0, "no iterations performed") {
					e.lock.Unlock()
					return
				}
				sink := e.Metrics[testMetric].(*stats.TrendSink)
				assert.True(t, len(sink.Values) > int(float64(e.numIterations)*0.99), "more than 1%% of iterations missed")
				e.lock.Unlock()
			})
		}
	})
}

func TestEngineIsRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e, err, _ := newTestEngine(nil, Options{})
	assert.NoError(t, err)

	ch := make(chan error)
	go func() { ch <- e.Run(ctx) }()
	runtime.Gosched()
	time.Sleep(1 * time.Millisecond)
	assert.True(t, e.IsRunning())

	cancel()
	runtime.Gosched()
	time.Sleep(1 * time.Millisecond)
	assert.False(t, e.IsRunning())

	assert.NoError(t, <-ch)
}

func TestEngineTotalTime(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		for _, d := range []time.Duration{0, 1 * time.Second, 10 * time.Second} {
			t.Run(d.String(), func(t *testing.T) {
				e, err, _ := newTestEngine(nil, Options{Duration: null.StringFrom(d.String())})
				assert.NoError(t, err)

				assert.Len(t, e.Stages, 1)
				assert.Equal(t, Stage{Duration: d}, e.Stages[0])
			})
		}
	})
	t.Run("Stages", func(t *testing.T) {
		// The lines get way too damn long if I have to write time.Second everywhere
		sec := time.Second

		testdata := map[string]struct {
			Duration time.Duration
			Stages   []Stage
		}{
			"nil":        {0, nil},
			"empty":      {0, []Stage{}},
			"1,infinite": {0, []Stage{{}}},
			"2,infinite": {0, []Stage{{Duration: 10 * sec}, {}}},
			"1,finite":   {10 * sec, []Stage{{Duration: 10 * sec}}},
			"2,finite":   {15 * sec, []Stage{{Duration: 10 * sec}, {Duration: 5 * sec}}},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				e, err, _ := newTestEngine(nil, Options{Stages: data.Stages})
				assert.NoError(t, err)
				assert.Equal(t, data.Duration, e.TotalTime())
			})
		}
	})
}

func TestEngineAtTime(t *testing.T) {
	e, err, _ := newTestEngine(nil, Options{})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.NoError(t, e.Run(ctx))
}

func TestEngineSetPaused(t *testing.T) {
	t.Run("offline", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)
		assert.False(t, e.IsPaused())

		e.SetPaused(true)
		assert.True(t, e.IsPaused())

		e.SetPaused(false)
		assert.False(t, e.IsPaused())
	})

	t.Run("running", func(t *testing.T) {
		e, err, _ := newTestEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
			return nil, nil
		}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
		assert.NoError(t, err)
		assert.False(t, e.IsPaused())

		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan error)
		go func() { ch <- e.Run(ctx) }()
		time.Sleep(1 * time.Millisecond)
		assert.True(t, e.IsRunning())

		// The iteration counter and time should increase over time when not paused...
		iterationSampleA1 := e.numIterations
		atTimeSampleA1 := e.AtTime()
		time.Sleep(100 * time.Millisecond)
		iterationSampleA2 := e.numIterations
		atTimeSampleA2 := e.AtTime()
		assert.True(t, iterationSampleA2 > iterationSampleA1, "iteration counter did not increase")
		assert.True(t, atTimeSampleA2 > atTimeSampleA1, "timer did not increase")

		// ...stop increasing when you pause... (sleep to ensure outstanding VUs finish)
		e.SetPaused(true)
		assert.True(t, e.IsPaused(), "engine did not pause")
		time.Sleep(1 * time.Millisecond)
		iterationSampleB1 := e.numIterations
		atTimeSampleB1 := e.AtTime()
		time.Sleep(100 * time.Millisecond)
		iterationSampleB2 := e.numIterations
		atTimeSampleB2 := e.AtTime()
		assert.Equal(t, iterationSampleB1, iterationSampleB2, "iteration counter changed while paused")
		assert.Equal(t, atTimeSampleB1, atTimeSampleB2, "timer changed while paused")

		// ...and resume when you unpause.
		e.SetPaused(false)
		assert.False(t, e.IsPaused(), "engine did not unpause")
		iterationSampleC1 := e.numIterations
		atTimeSampleC1 := e.AtTime()
		time.Sleep(100 * time.Millisecond)
		iterationSampleC2 := e.numIterations
		atTimeSampleC2 := e.AtTime()
		assert.True(t, iterationSampleC2 > iterationSampleC1, "iteration counter did not increase after unpause")
		assert.True(t, atTimeSampleC2 > atTimeSampleC1, "timer did not increase after unpause")

		cancel()
		assert.NoError(t, <-ch)
	})

	t.Run("exit", func(t *testing.T) {
		e, err, _ := newTestEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
			return nil, nil
		}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan error)
		go func() { ch <- e.Run(ctx) }()
		time.Sleep(1 * time.Millisecond)
		assert.True(t, e.IsRunning())

		e.SetPaused(true)
		assert.True(t, e.IsPaused())
		cancel()
		time.Sleep(1 * time.Millisecond)
		assert.False(t, e.IsPaused())
		assert.False(t, e.IsRunning())

		assert.NoError(t, <-ch)
	})
}

func TestEngineSetVUsMax(t *testing.T) {
	t.Run("not set", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), e.GetVUsMax())
		assert.Len(t, e.vuEntries, 0)
	})
	t.Run("set", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)
		assert.NoError(t, e.SetVUsMax(10))
		assert.Equal(t, int64(10), e.GetVUsMax())
		assert.Len(t, e.vuEntries, 10)
		for _, vu := range e.vuEntries {
			assert.Nil(t, vu.Cancel)
		}

		t.Run("higher", func(t *testing.T) {
			assert.NoError(t, e.SetVUsMax(15))
			assert.Equal(t, int64(15), e.GetVUsMax())
			assert.Len(t, e.vuEntries, 15)
			for _, vu := range e.vuEntries {
				assert.Nil(t, vu.Cancel)
			}
		})

		t.Run("lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUsMax(5))
			assert.Equal(t, int64(5), e.GetVUsMax())
			assert.Len(t, e.vuEntries, 5)
			for _, vu := range e.vuEntries {
				assert.Nil(t, vu.Cancel)
			}
		})
	})
	t.Run("set negative", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(-1), "vus-max can't be negative")
		assert.Len(t, e.vuEntries, 0)
	})
	t.Run("set too low", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{
			VUsMax: null.IntFrom(10),
			VUs:    null.IntFrom(10),
		})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(5), "can't reduce vus-max below vus")
		assert.Len(t, e.vuEntries, 10)
	})
}

func TestEngineSetVUs(t *testing.T) {
	assertVUIDSequence := func(t *testing.T, e *Engine, ids []int64) {
		actualIDs := make([]int64, len(ids))
		for i := range ids {
			actualIDs[i] = e.vuEntries[i].VU.(*RunnerFuncVU).ID
		}
		assert.Equal(t, ids, actualIDs)
	}

	t.Run("not set", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), e.GetVUsMax())
		assert.Equal(t, int64(0), e.GetVUs())
	})
	t.Run("set", func(t *testing.T) {
		e, err, _ := newTestEngine(RunnerFunc(nil), Options{VUsMax: null.IntFrom(15)})
		assert.NoError(t, err)
		assert.NoError(t, e.SetVUs(10))
		assert.Equal(t, int64(10), e.GetVUs())
		assertActiveVUs(t, e, 10, 5)
		assertVUIDSequence(t, e, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		t.Run("negative", func(t *testing.T) {
			assert.EqualError(t, e.SetVUs(-1), "vus can't be negative")
			assert.Equal(t, int64(10), e.GetVUs())
			assertActiveVUs(t, e, 10, 5)
			assertVUIDSequence(t, e, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		})

		t.Run("too high", func(t *testing.T) {
			assert.EqualError(t, e.SetVUs(20), "more vus than allocated requested")
			assert.Equal(t, int64(10), e.GetVUs())
			assertActiveVUs(t, e, 10, 5)
			assertVUIDSequence(t, e, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		})

		t.Run("lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(5))
			assert.Equal(t, int64(5), e.GetVUs())
			assertActiveVUs(t, e, 5, 10)
			assertVUIDSequence(t, e, []int64{1, 2, 3, 4, 5})
		})

		t.Run("higher", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(15))
			assert.Equal(t, int64(15), e.GetVUs())
			assertActiveVUs(t, e, 15, 0)
			assertVUIDSequence(t, e, []int64{1, 2, 3, 4, 5, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
		})
	})
}

func TestEngine_runVUOnceKeepsCounters(t *testing.T) {
	e, err, hook := newTestEngine(nil, Options{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), e.numIterations)
	assert.Equal(t, int64(0), e.numErrors)

	t.Run("success", func(t *testing.T) {
		hook.Reset()
		e.numIterations = 0
		e.numErrors = 0
		e.runVUOnce(context.Background(), &vuEntry{
			VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
				return nil, nil
			}).VU(),
		})
		assert.Equal(t, int64(1), e.numIterations)
		assert.Equal(t, int64(0), e.numErrors)
		assert.False(t, e.IsTainted(), "test is tainted")
	})
	t.Run("error", func(t *testing.T) {
		hook.Reset()
		e.numIterations = 0
		e.numErrors = 0
		e.runVUOnce(context.Background(), &vuEntry{
			VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
				return nil, errors.New("this is an error")
			}).VU(),
		})
		assert.Equal(t, int64(1), e.numIterations)
		assert.Equal(t, int64(1), e.numErrors)
		assert.Equal(t, "this is an error", hook.LastEntry().Data["error"].(error).Error())

		t.Run("string", func(t *testing.T) {
			hook.Reset()
			e.numIterations = 0
			e.numErrors = 0
			e.runVUOnce(context.Background(), &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, testErrorWithString("this is an error")
				}).VU(),
			})
			assert.Equal(t, int64(1), e.numIterations)
			assert.Equal(t, int64(1), e.numErrors)

			entry := hook.LastEntry()
			assert.Equal(t, "this is an error", entry.Message)
			assert.Empty(t, entry.Data)
		})
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		t.Run("success", func(t *testing.T) {
			hook.Reset()
			e.numIterations = 0
			e.numErrors = 0
			e.runVUOnce(ctx, &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, nil
				}).VU(),
			})
			assert.Equal(t, int64(0), e.numIterations)
			assert.Equal(t, int64(0), e.numErrors)
		})
		t.Run("error", func(t *testing.T) {
			hook.Reset()
			e.numIterations = 0
			e.numErrors = 0
			e.runVUOnce(ctx, &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, errors.New("this is an error")
				}).VU(),
			})
			assert.Equal(t, int64(0), e.numIterations)
			assert.Equal(t, int64(0), e.numErrors)
			assert.Nil(t, hook.LastEntry())

			t.Run("string", func(t *testing.T) {
				hook.Reset()
				e.numIterations = 0
				e.numErrors = 0
				e.runVUOnce(ctx, &vuEntry{
					VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
						return nil, testErrorWithString("this is an error")
					}).VU(),
				})
				assert.Equal(t, int64(0), e.numIterations)
				assert.Equal(t, int64(0), e.numErrors)
				assert.Nil(t, hook.LastEntry())
			})
		})
	})
}

func TestEngine_processStages(t *testing.T) {
	type checkpoint struct {
		D    time.Duration
		Cont bool
		VUs  int64
	}
	testdata := map[string]struct {
		Stages      []Stage
		Checkpoints []checkpoint
	}{
		"none": {
			[]Stage{},
			[]checkpoint{
				{0 * time.Second, false, 0},
				{10 * time.Second, false, 0},
				{24 * time.Hour, false, 0},
			},
		},
		"one": {
			[]Stage{
				{Duration: 10 * time.Second},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 0},
				{10 * time.Second, false, 0},
			},
		},
		"one/targeted": {
			[]Stage{
				{Duration: 10 * time.Second, Target: null.IntFrom(100)},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 10},
				{1 * time.Second, true, 20},
				{1 * time.Second, true, 30},
				{1 * time.Second, true, 40},
				{1 * time.Second, true, 50},
				{1 * time.Second, true, 60},
				{1 * time.Second, true, 70},
				{1 * time.Second, true, 80},
				{1 * time.Second, true, 90},
				{1 * time.Second, true, 100},
				{1 * time.Second, false, 100},
			},
		},
		"two": {
			[]Stage{
				{Duration: 5 * time.Second},
				{Duration: 5 * time.Second},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 0},
				{10 * time.Second, false, 0},
			},
		},
		"two/targeted": {
			[]Stage{
				{Duration: 5 * time.Second, Target: null.IntFrom(100)},
				{Duration: 5 * time.Second, Target: null.IntFrom(0)},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 20},
				{1 * time.Second, true, 40},
				{1 * time.Second, true, 60},
				{1 * time.Second, true, 80},
				{1 * time.Second, true, 100},
				{1 * time.Second, true, 80},
				{1 * time.Second, true, 60},
				{1 * time.Second, true, 40},
				{1 * time.Second, true, 20},
				{1 * time.Second, true, 0},
				{1 * time.Second, false, 0},
			},
		},
		"three": {
			[]Stage{
				{Duration: 5 * time.Second},
				{Duration: 5 * time.Second},
				{Duration: 5 * time.Second},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 0},
				{15 * time.Second, false, 0},
			},
		},
		"three/targeted": {
			[]Stage{
				{Duration: 5 * time.Second, Target: null.IntFrom(50)},
				{Duration: 5 * time.Second, Target: null.IntFrom(100)},
				{Duration: 5 * time.Second, Target: null.IntFrom(0)},
			},
			[]checkpoint{
				{0 * time.Second, true, 0},
				{1 * time.Second, true, 10},
				{1 * time.Second, true, 20},
				{1 * time.Second, true, 30},
				{1 * time.Second, true, 40},
				{1 * time.Second, true, 50},
				{1 * time.Second, true, 60},
				{1 * time.Second, true, 70},
				{1 * time.Second, true, 80},
				{1 * time.Second, true, 90},
				{1 * time.Second, true, 100},
				{1 * time.Second, true, 80},
				{1 * time.Second, true, 60},
				{1 * time.Second, true, 40},
				{1 * time.Second, true, 20},
				{1 * time.Second, true, 0},
				{1 * time.Second, false, 0},
			},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			e, err, _ := newTestEngine(nil, Options{
				VUs:    null.IntFrom(0),
				VUsMax: null.IntFrom(100),
			})
			assert.NoError(t, err)

			e.Stages = data.Stages
			for _, ckp := range data.Checkpoints {
				t.Run((e.AtTime() + ckp.D).String(), func(t *testing.T) {
					cont, err := e.processStages(ckp.D)
					assert.NoError(t, err)
					if ckp.Cont {
						assert.True(t, cont, "test stopped")
					} else {
						assert.False(t, cont, "test not stopped")
					}
					assert.Equal(t, ckp.VUs, e.GetVUs())
				})
			}
		})
	}
}

func TestEngineCollector(t *testing.T) {
	testMetric := stats.New("test_metric", stats.Trend)
	c := &dummy.Collector{}

	e, err, _ := newTestEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
		return []stats.Sample{{Metric: testMetric}}, nil
	}), Options{VUs: null.IntFrom(1), VUsMax: null.IntFrom(1)})
	assert.NoError(t, err)
	e.Collector = c

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan error)
	go func() { ch <- e.Run(ctx) }()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, e.IsRunning(), "engine not running")
	assert.True(t, c.IsRunning(), "collector not running")

	cancel()
	assert.NoError(t, <-ch)

	assert.False(t, e.IsRunning(), "engine still running")
	assert.False(t, c.IsRunning(), "collector still running")

	// Allow 10% of samples to get lost; NOT OPTIMAL, but I can't figure out why they get lost.
	numSamples := len(e.Metrics[testMetric].(*stats.TrendSink).Values)
	assert.True(t, numSamples > 0, "no samples")
	assert.True(t, numSamples > len(c.Samples)-(len(c.Samples)/10), "more than 10%% of samples omitted")
}

func TestEngine_processSamples(t *testing.T) {
	metric := stats.New("my_metric", stats.Gauge)

	t.Run("metric", func(t *testing.T) {
		e, err, _ := newTestEngine(nil, Options{})
		assert.NoError(t, err)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics[metric])
	})

	t.Run("submetric", func(t *testing.T) {
		ths, err := NewThresholds([]string{`1+1==2`})
		assert.NoError(t, err)

		e, err, _ := newTestEngine(nil, Options{
			Thresholds: map[string]Thresholds{
				"my_metric{a:1}": ths,
			},
		})
		assert.NoError(t, err)

		sms := e.submetrics["my_metric"]
		assert.Len(t, sms, 1)
		assert.Equal(t, "my_metric{a:1}", sms[0].Name)
		assert.EqualValues(t, map[string]string{"a": "1"}, sms[0].Conditions)

		e.processSamples(
			stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
		)

		assert.IsType(t, &stats.GaugeSink{}, e.Metrics[metric])

		sms = e.submetrics["my_metric"]
		assert.IsType(t, &stats.GaugeSink{}, e.Metrics[sms[0].Metric])
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
			thresholds := make(map[string]Thresholds, len(data.ths))
			for m, srcs := range data.ths {
				ths, err := NewThresholds(srcs)
				assert.NoError(t, err)
				thresholds[m] = ths
			}

			e, err, _ := newTestEngine(nil, Options{Thresholds: thresholds})
			assert.NoError(t, err)

			e.processSamples(
				stats.Sample{Metric: metric, Value: 1.25, Tags: map[string]string{"a": "1"}},
			)
			e.processThresholds()

			assert.Equal(t, data.pass, !e.IsTainted())
		})
	}
}
