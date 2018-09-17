/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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
package http

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlotLimiterSingleSlot(t *testing.T) {
	t.Parallel()
	l := NewSlotLimiter(1)
	l.Begin()
	done := false
	go func() {
		done = true
		l.End()
	}()
	l.Begin()
	assert.True(t, done)
	l.End()
}

func TestSlotLimiterUnlimited(t *testing.T) {
	t.Parallel()
	l := NewSlotLimiter(0)
	l.Begin()
	l.Begin()
	l.Begin()
}
func TestSlotLimiters(t *testing.T) {
	t.Parallel()
	testCases := []struct{ limit, launches, expMid int }{
		{0, 0, 0},
		{0, 1, 1},
		{0, 5, 5},
		{1, 5, 1},
		{2, 5, 2},
		{5, 6, 5},
		{6, 5, 5},
		{10, 7, 7},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("limit=%d,launches=%d", tc.limit, tc.launches), func(t *testing.T) {
			t.Parallel()
			l := NewSlotLimiter(tc.limit)
			wg := sync.WaitGroup{}

			if tc.limit == 0 {
				wg.Add(tc.launches)
			} else if tc.launches < tc.limit {
				wg.Add(tc.launches)
			} else {
				wg.Add(tc.limit)
			}

			var counter uint32

			for i := 0; i < tc.launches; i++ {
				go func() {
					l.Begin()
					atomic.AddUint32(&counter, 1)
					wg.Done()
				}()
			}
			wg.Wait()
			assert.Equal(t, uint32(tc.expMid), atomic.LoadUint32(&counter))

			if tc.limit != 0 && tc.limit < tc.launches {
				wg.Add(tc.launches - tc.limit)
				for i := 0; i < tc.launches; i++ {
					l.End()
				}
				wg.Wait()
				assert.Equal(t, uint32(tc.launches), atomic.LoadUint32(&counter))
			}
		})
	}
}

func TestMultiSlotLimiter(t *testing.T) {
	t.Parallel()
	t.Run("0", func(t *testing.T) {
		l := NewMultiSlotLimiter(0)
		assert.Nil(t, l.Slot("test"))
	})
	t.Run("1", func(t *testing.T) {
		l := NewMultiSlotLimiter(1)
		assert.Equal(t, l.Slot("test"), l.Slot("test"))
		assert.NotNil(t, l.Slot("test"))
	})
	t.Run("2", func(t *testing.T) {
		l := NewMultiSlotLimiter(1)
		wg := sync.WaitGroup{}
		wg.Add(2)

		var s1, s2 SlotLimiter
		go func() {
			s1 = l.Slot("ctest")
			wg.Done()
		}()
		go func() {
			s2 = l.Slot("ctest")
			wg.Done()
		}()
		wg.Wait()

		assert.NotNil(t, s1)
		assert.Equal(t, s1, s2)
		assert.Equal(t, s1, l.Slot("ctest"))
		assert.NotEqual(t, s1, l.Slot("dtest"))
		assert.NotNil(t, l.Slot("dtest"))
	})
}
