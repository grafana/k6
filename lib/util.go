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
	// "math"
	"time"
)

// StageAt returns the stage at the specified offset (in nanoseconds) and the time remaining of
// said stage. If the interval is past the end of the test, an empty stage and 0 is returned.
func StageAt(stages []Stage, offset time.Duration) (s Stage, stageLeft time.Duration, ok bool) {
	var counter time.Duration
	for _, stage := range stages {
		counter += time.Duration(stage.Duration.Int64)
		if counter >= offset {
			return stage, counter - offset, true
		}
	}
	return stages[len(stages)-1], 0, false
}

// Lerp is a linear interpolation between two values x and y, returning the value at the point t,
// where t is a fraction in the range [0.0 - 1.0].
func Lerp(x, y int64, t float64) int64 {
	return x + int64(t*float64(y-x))
}
