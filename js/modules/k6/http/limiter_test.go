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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlotLimiter(t *testing.T) {
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

func TestMultiSlotLimiter(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		l := NewMultiSlotLimiter(0)
		assert.Nil(t, l.Slot("test"))
	})
	t.Run("1", func(t *testing.T) {
		l := NewMultiSlotLimiter(1)
		assert.Equal(t, l.Slot("test"), l.Slot("test"))
		assert.NotNil(t, l.Slot("test"))
	})
}
