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
	"fmt"
	logtest "github.com/Sirupsen/logrus/hooks/test"
	"github.com/loadimpact/k6/stats"
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

// Helper for asserting the number of active/dead VUs.
func assertActiveVUs(t *testing.T, e *Engine, active, dead int) {
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

func TestNewEngine(t *testing.T) {
	_, err := NewEngine(nil, Options{})
	assert.NoError(t, err)
}

func TestNewEngineOptions(t *testing.T) {
	t.Run("VUsMax", func(t *testing.T) {
		t.Run("not set", func(t *testing.T) {
			e, err := NewEngine(nil, Options{})
			assert.NoError(t, err)
			assert.Equal(t, int64(0), e.GetVUsMax())
			assert.Equal(t, int64(0), e.GetVUs())
		})
		t.Run("set", func(t *testing.T) {
			e, err := NewEngine(nil, Options{
				VUsMax: null.IntFrom(10),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.GetVUsMax())
			assert.Equal(t, int64(0), e.GetVUs())
		})
	})
	t.Run("VUs", func(t *testing.T) {
		t.Run("no max", func(t *testing.T) {
			_, err := NewEngine(nil, Options{
				VUs: null.IntFrom(10),
			})
			assert.EqualError(t, err, "more vus than allocated requested")
		})
		t.Run("max too low", func(t *testing.T) {
			_, err := NewEngine(nil, Options{
				VUsMax: null.IntFrom(1),
				VUs:    null.IntFrom(10),
			})
			assert.EqualError(t, err, "more vus than allocated requested")
		})
		t.Run("max higher", func(t *testing.T) {
			e, err := NewEngine(nil, Options{
				VUsMax: null.IntFrom(10),
				VUs:    null.IntFrom(1),
			})
			assert.NoError(t, err)
			assert.Equal(t, int64(10), e.GetVUsMax())
			assert.Equal(t, int64(1), e.GetVUs())
		})
		t.Run("max just right", func(t *testing.T) {
			e, err := NewEngine(nil, Options{
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
			e, err := NewEngine(nil, Options{})
			assert.NoError(t, err)
			assert.False(t, e.IsPaused())
		})
		t.Run("false", func(t *testing.T) {
			e, err := NewEngine(nil, Options{
				Paused: null.BoolFrom(false),
			})
			assert.NoError(t, err)
			assert.False(t, e.IsPaused())
		})
		t.Run("true", func(t *testing.T) {
			e, err := NewEngine(nil, Options{
				Paused: null.BoolFrom(true),
			})
			assert.NoError(t, err)
			assert.True(t, e.IsPaused())
		})
	})
}

func TestEngineRun(t *testing.T) {
	t.Run("exits with context", func(t *testing.T) {
		startTime := time.Now()
		duration := 100 * time.Millisecond
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)

		ctx, _ := context.WithTimeout(context.Background(), duration)
		assert.NoError(t, e.Run(ctx))
		assert.WithinDuration(t, startTime.Add(duration), time.Now(), 100*time.Millisecond)
	})
	t.Run("terminates subctx", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
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
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)

		d := 50 * time.Millisecond
		e.Stages = []Stage{Stage{Duration: d}}
		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background()))
		assert.WithinDuration(t, startTime.Add(d), startTime.Add(e.AtTime()), 2*TickRate)
	})
	t.Run("collects samples", func(t *testing.T) {
		testMetric := stats.New("test_metric", stats.Trend)

		errors := map[string]error{
			"nil":   nil,
			"error": errors.New("error"),
			"taint": ErrVUWantsTaint,
		}
		for name, reterr := range errors {
			t.Run(name, func(t *testing.T) {
				e, err := NewEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return []stats.Sample{stats.Sample{Metric: testMetric, Value: 1.0}}, reterr
				}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
				assert.NoError(t, err)

				ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
				assert.NoError(t, e.Run(ctx))
				if !assert.True(t, e.numIterations > 0, "no iterations performed") {
					return
				}

				sink := e.Metrics[testMetric].(*stats.TrendSink)
				assert.True(t, len(sink.Values) > int(float64(e.numIterations)*0.99), "more than 1%% of iterations missed")
			})
		}
	})
}

func TestEngineIsRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e, err := NewEngine(nil, Options{})
	assert.NoError(t, err)

	go func() { assert.NoError(t, e.Run(ctx)) }()
	runtime.Gosched()
	time.Sleep(1 * time.Millisecond)
	assert.True(t, e.IsRunning())

	cancel()
	runtime.Gosched()
	time.Sleep(1 * time.Millisecond)
	assert.False(t, e.IsRunning())
}

func TestEngineTotalTime(t *testing.T) {
	t.Run("Duration", func(t *testing.T) {
		for _, d := range []time.Duration{0, 1 * time.Second, 10 * time.Second} {
			t.Run(d.String(), func(t *testing.T) {
				e, err := NewEngine(nil, Options{Duration: null.StringFrom(d.String())})
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
			"1,infinite": {0, []Stage{Stage{}}},
			"2,infinite": {0, []Stage{Stage{Duration: 10 * sec}, Stage{}}},
			"1,finite":   {10 * sec, []Stage{Stage{Duration: 10 * sec}}},
			"2,finite":   {15 * sec, []Stage{Stage{Duration: 10 * sec}, Stage{Duration: 5 * sec}}},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				e, err := NewEngine(nil, Options{Stages: data.Stages})
				assert.NoError(t, err)
				assert.Equal(t, data.Duration, e.TotalTime())
			})
		}
	})
}

func TestEngineAtTime(t *testing.T) {
	e, err := NewEngine(nil, Options{})
	assert.NoError(t, err)

	d := 50 * time.Millisecond
	ctx, _ := context.WithTimeout(context.Background(), d)
	startTime := time.Now()
	assert.NoError(t, e.Run(ctx))
	assert.WithinDuration(t, startTime.Add(d), startTime.Add(e.AtTime()), 2*TickRate)
}

func TestEngineSetPaused(t *testing.T) {
	t.Run("offline", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.False(t, e.IsPaused())

		e.SetPaused(true)
		assert.True(t, e.IsPaused())

		e.SetPaused(false)
		assert.False(t, e.IsPaused())
	})

	t.Run("running", func(t *testing.T) {
		e, err := NewEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
			return nil, nil
		}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
		assert.NoError(t, err)
		assert.False(t, e.IsPaused())

		ctx, cancel := context.WithCancel(context.Background())
		go func() { assert.NoError(t, e.Run(ctx)) }()
		defer cancel()
		time.Sleep(1 * time.Millisecond)
		assert.True(t, e.IsRunning())

		// The iteration counter and time should increase over time when not paused...
		iterationSampleA1 := e.numIterations
		atTimeSampleA1 := e.AtTime()
		time.Sleep(1 * time.Millisecond)
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
		time.Sleep(1 * time.Millisecond)
		iterationSampleB2 := e.numIterations
		atTimeSampleB2 := e.AtTime()
		assert.Equal(t, iterationSampleB1, iterationSampleB2, "iteration counter changed while paused")
		assert.Equal(t, atTimeSampleB1, atTimeSampleB2, "timer changed while paused")

		// ...and resume when you unpause.
		e.SetPaused(false)
		assert.False(t, e.IsPaused(), "engine did not unpause")
		iterationSampleC1 := e.numIterations
		atTimeSampleC1 := e.AtTime()
		time.Sleep(1 * time.Millisecond)
		iterationSampleC2 := e.numIterations
		atTimeSampleC2 := e.AtTime()
		assert.True(t, iterationSampleC2 > iterationSampleC1, "iteration counter did not increase after unpause")
		assert.True(t, atTimeSampleC2 > atTimeSampleC1, "timer did not increase after unpause")
	})

	t.Run("exit", func(t *testing.T) {
		e, err := NewEngine(RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
			return nil, nil
		}), Options{VUsMax: null.IntFrom(1), VUs: null.IntFrom(1)})
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go func() { assert.NoError(t, e.Run(ctx)) }()
		time.Sleep(1 * time.Millisecond)
		assert.True(t, e.IsRunning())

		e.SetPaused(true)
		assert.True(t, e.IsPaused())
		cancel()
		time.Sleep(1 * time.Millisecond)
		assert.False(t, e.IsPaused())
		assert.False(t, e.IsRunning())
	})
}

func TestEngineSetVUsMax(t *testing.T) {
	t.Run("not set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), e.GetVUsMax())
		assert.Len(t, e.vuEntries, 0)
	})
	t.Run("set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
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
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(-1), "vus-max can't be negative")
		assert.Len(t, e.vuEntries, 0)
	})
	t.Run("set too low", func(t *testing.T) {
		e, err := NewEngine(nil, Options{
			VUsMax: null.IntFrom(10),
			VUs:    null.IntFrom(10),
		})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(5), "can't reduce vus-max below vus")
		assert.Len(t, e.vuEntries, 10)
	})
}

func TestEngineSetVUs(t *testing.T) {
	t.Run("not set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), e.GetVUsMax())
		assert.Equal(t, int64(0), e.GetVUs())
	})
	t.Run("set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{VUsMax: null.IntFrom(15)})
		assert.NoError(t, err)
		assert.NoError(t, e.SetVUs(10))
		assert.Equal(t, int64(10), e.GetVUs())
		assertActiveVUs(t, e, 10, 5)

		t.Run("negative", func(t *testing.T) {
			assert.EqualError(t, e.SetVUs(-1), "vus can't be negative")
			assert.Equal(t, int64(10), e.GetVUs())
			assertActiveVUs(t, e, 10, 5)
		})

		t.Run("too high", func(t *testing.T) {
			assert.EqualError(t, e.SetVUs(20), "more vus than allocated requested")
			assert.Equal(t, int64(10), e.GetVUs())
			assertActiveVUs(t, e, 10, 5)
		})

		t.Run("lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(5))
			assert.Equal(t, int64(5), e.GetVUs())
			assertActiveVUs(t, e, 5, 10)
		})

		t.Run("higher", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(15))
			assert.Equal(t, int64(15), e.GetVUs())
			assertActiveVUs(t, e, 15, 0)
		})
	})
}

func TestEngineIsTainted(t *testing.T) {
	testdata := []struct {
		I      int64
		T      int64
		Expect bool
	}{
		{1, 0, false},
		{1, 1, true},
	}

	for _, data := range testdata {
		t.Run(fmt.Sprintf("i=%d,t=%d", data.I, data.T), func(t *testing.T) {
			e, err := NewEngine(nil, Options{})
			assert.NoError(t, err)

			e.numIterations = data.I
			e.numTaints = data.T
			assert.Equal(t, data.Expect, e.IsTainted())
		})
	}
}

func TestEngine_runVUOnceKeepsCounters(t *testing.T) {
	e, err := NewEngine(nil, Options{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), e.numIterations)
	assert.Equal(t, int64(0), e.numTaints)

	t.Run("success", func(t *testing.T) {
		e.numIterations = 0
		e.numTaints = 0
		e.runVUOnce(context.Background(), &vuEntry{
			VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
				return nil, nil
			}),
		})
		assert.Equal(t, int64(1), e.numIterations)
		assert.Equal(t, int64(0), e.numTaints)
		assert.False(t, e.IsTainted(), "test is tainted")
	})
	t.Run("error", func(t *testing.T) {
		hook := logtest.NewGlobal()
		defer hook.Reset()

		e.numIterations = 0
		e.numTaints = 0
		e.runVUOnce(context.Background(), &vuEntry{
			VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
				return nil, errors.New("this is an error")
			}),
		})
		assert.Equal(t, int64(1), e.numIterations)
		assert.Equal(t, int64(1), e.numTaints)
		assert.True(t, e.IsTainted(), "test is not tainted")
		assert.Equal(t, "this is an error", hook.LastEntry().Data["error"].(error).Error())

		t.Run("string", func(t *testing.T) {
			e.numIterations = 0
			e.numTaints = 0
			e.runVUOnce(context.Background(), &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, testErrorWithString("this is an error")
				}),
			})
			assert.Equal(t, int64(1), e.numIterations)
			assert.Equal(t, int64(1), e.numTaints)
			assert.True(t, e.IsTainted(), "test is not tainted")

			entry := hook.LastEntry()
			assert.Equal(t, "this is an error", entry.Message)
			assert.Empty(t, entry.Data)
		})
	})
	t.Run("taint", func(t *testing.T) {
		hook := logtest.NewGlobal()
		defer hook.Reset()

		e.numIterations = 0
		e.numTaints = 0
		e.runVUOnce(context.Background(), &vuEntry{
			VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
				return nil, ErrVUWantsTaint
			}),
		})
		assert.Equal(t, int64(1), e.numIterations)
		assert.Equal(t, int64(1), e.numTaints)
		assert.True(t, e.IsTainted(), "test is not tainted")

		assert.Nil(t, hook.LastEntry())
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		t.Run("success", func(t *testing.T) {
			e.numIterations = 0
			e.numTaints = 0
			e.runVUOnce(ctx, &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, nil
				}),
			})
			assert.Equal(t, int64(0), e.numIterations)
			assert.Equal(t, int64(0), e.numTaints)
			assert.False(t, e.IsTainted(), "test is tainted")
		})
		t.Run("error", func(t *testing.T) {
			hook := logtest.NewGlobal()
			defer hook.Reset()

			e.numIterations = 0
			e.numTaints = 0
			e.runVUOnce(ctx, &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, errors.New("this is an error")
				}),
			})
			assert.Equal(t, int64(0), e.numIterations)
			assert.Equal(t, int64(0), e.numTaints)
			assert.False(t, e.IsTainted(), "test is tainted")
			assert.Nil(t, hook.LastEntry())

			t.Run("string", func(t *testing.T) {
				e.numIterations = 0
				e.numTaints = 0
				e.runVUOnce(ctx, &vuEntry{
					VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
						return nil, testErrorWithString("this is an error")
					}),
				})
				assert.Equal(t, int64(0), e.numIterations)
				assert.Equal(t, int64(0), e.numTaints)
				assert.False(t, e.IsTainted(), "test is tainted")

				assert.Nil(t, hook.LastEntry())
			})
		})
		t.Run("taint", func(t *testing.T) {
			hook := logtest.NewGlobal()
			defer hook.Reset()

			e.numIterations = 0
			e.numTaints = 0
			e.runVUOnce(ctx, &vuEntry{
				VU: RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
					return nil, ErrVUWantsTaint
				}),
			})
			assert.Equal(t, int64(0), e.numIterations)
			assert.Equal(t, int64(0), e.numTaints)
			assert.False(t, e.IsTainted(), "test is tainted")

			assert.Nil(t, hook.LastEntry())
		})
	})
}
