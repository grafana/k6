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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorIsRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e := New(nil)

	go e.Run(ctx, nil)
	for !e.IsRunning() {
	}
	cancel()
	for e.IsRunning() {
	}
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
