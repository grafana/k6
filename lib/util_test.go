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
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestLerp(t *testing.T) {
	// data[x][y][t] = v
	data := map[int64]map[int64]map[float64]int64{
		0: map[int64]map[float64]int64{
			0:   map[float64]int64{0.0: 0, 0.10: 0, 0.5: 0, 1.0: 0},
			100: map[float64]int64{0.0: 0, 0.10: 10, 0.5: 50, 1.0: 100},
			500: map[float64]int64{0.0: 0, 0.10: 50, 0.5: 250, 1.0: 500},
		},
		100: map[int64]map[float64]int64{
			200: map[float64]int64{0.0: 100, 0.1: 110, 0.5: 150, 1.0: 200},
			0:   map[float64]int64{0.0: 100, 0.1: 90, 0.5: 50, 1.0: 0},
		},
	}

	for x, data := range data {
		t.Run("x="+strconv.FormatInt(x, 10), func(t *testing.T) {
			for y, data := range data {
				t.Run("y="+strconv.FormatInt(y, 10), func(t *testing.T) {
					for t_, x1 := range data {
						t.Run("t="+strconv.FormatFloat(t_, 'f', 2, 64), func(t *testing.T) {
							assert.Equal(t, x1, Lerp(x, y, t_))
						})
					}
				})
			}
		})
	}
}

func TestClampf(t *testing.T) {
	testdata := map[float64]map[struct {
		Min, Max float64
	}]float64{
		-1.0: {
			{0.0, 1.0}: 0.0,
			{0.5, 1.0}: 0.5,
			{1.0, 1.0}: 1.0,
			{0.0, 0.5}: 0.0,
		},
		0.0: {
			{0.0, 1.0}: 0.0,
			{0.5, 1.0}: 0.5,
			{1.0, 1.0}: 1.0,
			{0.0, 0.5}: 0.0,
		},
		0.5: {
			{0.0, 1.0}: 0.5,
			{0.5, 1.0}: 0.5,
			{1.0, 1.0}: 1.0,
			{0.0, 0.5}: 0.5,
		},
		1.0: {
			{0.0, 1.0}: 1.0,
			{0.5, 1.0}: 1.0,
			{1.0, 1.0}: 1.0,
			{0.0, 0.5}: 0.5,
		},
		2.0: {
			{0.0, 1.0}: 1.0,
			{0.5, 1.0}: 1.0,
			{1.0, 1.0}: 1.0,
			{0.0, 0.5}: 0.5,
		},
	}

	for val, ranges := range testdata {
		t.Run(fmt.Sprintf("val=%.1f", val), func(t *testing.T) {
			for r, result := range ranges {
				t.Run(fmt.Sprintf("min=%.1f,max=%.1f", r.Min, r.Max), func(t *testing.T) {
					assert.Equal(t, result, Clampf(val, r.Min, r.Max))
				})
			}
		})
	}
}
