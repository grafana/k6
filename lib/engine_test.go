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
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
	"runtime"
	"testing"
	"time"
)

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
		e.Run(ctx)

		assert.NotEqual(t, subctx, e.subctx, "subcontext not changed")
		select {
		case <-subctx.Done():
		default:
			assert.Fail(t, "context was not terminated")
		}
	})
}

func TestEngineIsRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e, err := NewEngine(nil, Options{})
	assert.NoError(t, err)

	go e.Run(ctx)
	runtime.Gosched()
	assert.True(t, e.IsRunning())

	cancel()
	runtime.Gosched()
	assert.False(t, e.IsRunning())
}

func TestEngineSetPaused(t *testing.T) {
	e, err := NewEngine(nil, Options{})
	assert.NoError(t, err)
	assert.False(t, e.IsPaused())

	e.SetPaused(true)
	assert.True(t, e.IsPaused())

	e.SetPaused(false)
	assert.False(t, e.IsPaused())
}

func TestEngineSetVUsMax(t *testing.T) {
	t.Run("not set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), e.GetVUsMax())
	})
	t.Run("set", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.NoError(t, e.SetVUsMax(10))
		assert.Equal(t, int64(10), e.GetVUsMax())
	})
	t.Run("set negative", func(t *testing.T) {
		e, err := NewEngine(nil, Options{})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(-1), "vus-max can't be negative")
	})
	t.Run("set too low", func(t *testing.T) {
		e, err := NewEngine(nil, Options{
			VUsMax: null.IntFrom(10),
			VUs:    null.IntFrom(10),
		})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUsMax(5), "can't reduce vus-max below vus")
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
		e, err := NewEngine(nil, Options{VUsMax: null.IntFrom(10)})
		assert.NoError(t, err)
		assert.NoError(t, e.SetVUs(10))
		assert.Equal(t, int64(10), e.GetVUs())
	})
	t.Run("set too high", func(t *testing.T) {
		e, err := NewEngine(nil, Options{VUsMax: null.IntFrom(10)})
		assert.NoError(t, err)
		assert.EqualError(t, e.SetVUs(20), "more vus than allocated requested")
	})
}
