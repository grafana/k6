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
	"time"

	"github.com/loadimpact/k6/lib"
	"gopkg.in/guregu/null.v3"
)

// Returns the VU count and whether to keep going at the specified time.
func ProcessStages(startVUs int64, stages []lib.Stage, t time.Duration) (null.Int, bool) {
	vus := null.NewInt(startVUs, false)

	var start time.Duration
	for _, stage := range stages {
		// Infinite stages keep running forever, with the last valid end point, or its own target.
		if !stage.Duration.Valid {
			if stage.Target.Valid {
				vus = stage.Target
			}
			return vus, true
		}

		// If the stage has already ended, still record the end VU count for interpolation.
		end := start + time.Duration(stage.Duration.Duration)
		if end < t {
			if stage.Target.Valid {
				vus = stage.Target
			}
			start = end
			continue
		}

		// If there's a VU target, use linear interpolation to reach it.
		if stage.Target.Valid {
			prog := lib.Clampf(float64(t-start)/float64(stage.Duration.Duration), 0.0, 1.0)
			vus = null.IntFrom(lib.Lerp(vus.Int64, stage.Target.Int64, prog))
		}

		// We found a stage, so keep running.
		return vus, true
	}
	return vus, false
}
