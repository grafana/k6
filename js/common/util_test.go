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

package common

import (
	"errors"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestThrow(t *testing.T) {
	rt := goja.New()
	fn1, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, errors.New("aaaa")) }))
	if assert.True(t, ok, "fn1 is invalid") {
		_, err := fn1(goja.Undefined())
		assert.EqualError(t, err, "aaaa")

		fn2, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, err) }))
		if assert.True(t, ok, "fn1 is invalid") {
			_, err := fn2(goja.Undefined())
			assert.EqualError(t, err, "aaaa")
		}
	}
}
