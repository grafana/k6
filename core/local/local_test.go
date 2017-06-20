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
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v3"
)

func TestExecutorRun(t *testing.T) {
	ch := make(chan struct{})
	e := New(lib.RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
		select {
		case ch <- struct{}{}:
		case <-ctx.Done():
		}
		return nil, nil
	}))
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))

	ctx, cancel := context.WithCancel(context.Background())
	err := make(chan error)
	go func() { err <- e.Run(ctx, nil) }()
	for i := 0; i < 10; i++ {
		<-ch
	}
	cancel()
	assert.NoError(t, <-err)
	assert.Equal(t, int64(10), e.GetIterations())
}

func TestExecutorEndTime(t *testing.T) {
	e := New(nil)
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))
	e.SetEndTime(lib.NullDurationFrom(1 * time.Second))
	assert.Equal(t, lib.NullDurationFrom(1*time.Second), e.GetEndTime())

	startTime := time.Now()
	assert.NoError(t, e.Run(context.Background(), nil))
	assert.True(t, time.Now().After(startTime.Add(1*time.Second)), "test did not take 1s")
}

func TestExecutorEndIterations(t *testing.T) {
	var i int64
	e := New(lib.RunnerFunc(func(ctx context.Context) ([]stats.Sample, error) {
		atomic.AddInt64(&i, 1)
		return nil, nil
	}))
	assert.NoError(t, e.SetVUsMax(10))
	assert.NoError(t, e.SetVUs(10))
	e.SetEndIterations(null.IntFrom(100))
	assert.Equal(t, null.IntFrom(100), e.GetEndIterations())
	assert.NoError(t, e.Run(context.Background(), nil))
	assert.Equal(t, int64(100), e.GetIterations())
	assert.Equal(t, int64(100), i)
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
		e := New(nil)
		e.ctx = context.Background()

		assert.NoError(t, e.SetVUsMax(100))
		assert.Equal(t, int64(100), e.GetVUsMax())

		assert.NoError(t, e.SetVUs(50))
		assert.Equal(t, int64(50), e.GetVUs())

		assert.NoError(t, e.SetVUs(100))
		assert.Equal(t, int64(100), e.GetVUs())

		t.Run("Lower", func(t *testing.T) {
			assert.NoError(t, e.SetVUs(50))
			assert.Equal(t, int64(50), e.GetVUs())
		})
	})
}
