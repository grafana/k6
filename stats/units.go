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

package stats

import (
	"time"
)

const timeUnit = time.Millisecond

// D formats a duration for emission.
// The reverse of D() is ToD().
func D(d time.Duration) float64 {
	return float64(d) / float64(timeUnit)
}

// ToD converts an emitted duration to a time.Duration.
// The reverse of ToD() is D().
func ToD(d float64) time.Duration {
	return time.Duration(d * float64(timeUnit))
}

// B formats a boolean value for emission.
func B(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
