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
	"sync/atomic"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v3"
)

func TestExecutorRun(t *testing.T) {
	e := New(nil)
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))

	ctx, cancel := context.WithCancel(context.Background())
	err := make(chan error, 1)
	go func() { err <- e.Run(ctx, nil) }()
	cancel()
	assert.NoError(t, <-err)
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
			e := New(nil)
			assert.NoError(t, e.SetVUsMax(10))
			e.SetStages(data.Stages)
			assert.NoError(t, e.Run(context.Background(), nil))
			assert.True(t, e.GetTime() >= data.Duration)
		})
	}
}

func TestExecutorEndTime(t *testing.T) {
	e := New(nil)
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))
	e.SetEndTime(types.NullDurationFrom(1 * time.Second))
	assert.Equal(t, types.NullDurationFrom(1*time.Second), e.GetEndTime())

	startTime := time.Now()
	assert.NoError(t, e.Run(context.Background(), nil))
	assert.True(t, time.Now().After(startTime.Add(1*time.Second)), "test did not take 1s")

	t.Run("Runtime Errors", func(t *testing.T) {
		e := New(lib.MiniRunner{Fn: func(ctx context.Context) ([]stats.Sample, error) {
			return nil, errors.New("hi")
		}})
		assert.NoError(t, e.SetVUsMax(10))
		assert.NoError(t, e.SetVUs(10))
		e.SetEndTime(types.NullDurationFrom(100 * time.Millisecond))
		assert.Equal(t, types.NullDurationFrom(100*time.Millisecond), e.GetEndTime())

		l, hook := logtest.NewNullLogger()
		e.SetLogger(l)

		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background(), nil))
		assert.True(t, time.Now().After(startTime.Add(100*time.Millisecond)), "test did not take 100ms")

		assert.NotEmpty(t, hook.Entries)
		for _, e := range hook.Entries {
			assert.Equal(t, "hi", e.Message)
		}
	})

	t.Run("End Errors", func(t *testing.T) {
		e := New(lib.MiniRunner{Fn: func(ctx context.Context) ([]stats.Sample, error) {
			<-ctx.Done()
			return nil, errors.New("hi")
		}})
		assert.NoError(t, e.SetVUsMax(10))
		assert.NoError(t, e.SetVUs(10))
		e.SetEndTime(types.NullDurationFrom(100 * time.Millisecond))
		assert.Equal(t, types.NullDurationFrom(100*time.Millisecond), e.GetEndTime())

		l, hook := logtest.NewNullLogger()
		e.SetLogger(l)

		startTime := time.Now()
		assert.NoError(t, e.Run(context.Background(), nil))
		assert.True(t, time.Now().After(startTime.Add(100*time.Millisecond)), "test did not take 100ms")

		assert.Empty(t, hook.Entries)
	})
}

func TestExecutorEndIterations(t *testing.T) {
	metric := &stats.Metric{Name: "test_metric"}

	var i int64
	e := New(lib.MiniRunner{Fn: func(ctx context.Context) ([]stats.Sample, error) {
		select {
		case <-ctx.Done():
		default:
			atomic.AddInt64(&i, 1)
		}
		return []stats.Sample{{Metric: metric, Value: 1.0}}, nil
	}})
	assert.NoError(t, e.SetVUsMax(1))
	assert.NoError(t, e.SetVUs(1))
	e.SetEndIterations(null.IntFrom(100))
	assert.Equal(t, null.IntFrom(100), e.GetEndIterations())

	samples := make(chan []stats.Sample, 101)
	assert.NoError(t, e.Run(context.Background(), samples))
	assert.Equal(t, int64(100), e.GetIterations())
	assert.Equal(t, int64(100), i)

	for i := 0; i < 100; i++ {
		samples := <-samples
		if assert.Len(t, samples, 2) {
			assert.Equal(t, stats.Sample{Metric: metric, Value: 1.0}, samples[0])
			assert.Equal(t, metrics.Iterations, samples[1].Metric)
			assert.Equal(t, float64(1), samples[1].Value)
		}
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
		e := New(lib.MiniRunner{Fn: func(ctx context.Context) ([]stats.Sample, error) {
			return nil, nil
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
