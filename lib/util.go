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
	"strings"
)

// Splits a string in the form "key=value".
func SplitKV(s string) (key, value string) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// Lerp is a linear interpolation between two values x and y, returning the value at the point t,
// where t is a fraction in the range [0.0 - 1.0].
func Lerp(x, y int64, t float64) int64 {
	return x + int64(t*float64(y-x))
}

// Clampf returns the given value, "clamped" to the range [min, max].
func Clampf(val, min, max float64) float64 {
	switch {
	case val < min:
		return min
	case val > max:
		return max
	default:
		return val
	}
}

// Returns the maximum value of a and b.
func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Returns the minimum value of a and b.
func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
