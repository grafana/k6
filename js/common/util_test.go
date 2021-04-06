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
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestThrow(t *testing.T) {
	rt := goja.New()
	fn1, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, errors.New("aaaa")) }))
	if assert.True(t, ok, "fn1 is invalid") {
		_, err := fn1(goja.Undefined())
		assert.EqualError(t, err, "GoError: aaaa")

		fn2, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, err) }))
		if assert.True(t, ok, "fn1 is invalid") {
			_, err := fn2(goja.Undefined())
			assert.EqualError(t, err, "GoError: aaaa")
		}
	}
}

func TestToBytes(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	b := []byte("hello")
	testCases := []struct {
		in     interface{}
		expOut []byte
		expErr string
	}{
		{b, b, ""},
		{"hello", b, ""},
		{rt.NewArrayBuffer(b), b, ""},
		{struct{}{}, nil, "invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			out, err := ToBytes(tc.in)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			assert.Equal(t, tc.expOut, out)
		})
	}
}

func TestToString(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	s := "hello"
	testCases := []struct {
		in             interface{}
		expOut, expErr string
	}{
		{s, s, ""},
		{"hello", s, ""},
		{rt.NewArrayBuffer([]byte(s)), s, ""},
		{struct{}{}, "", "invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for _, tc := range testCases { //nolint: paralleltest // false positive?
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			t.Parallel()
			out, err := ToString(tc.in)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			assert.Equal(t, tc.expOut, out)
		})
	}
}
