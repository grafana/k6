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

package core

import (
	"github.com/loadimpact/k6/lib"
)

// Returns the total sum of time taken by the given set of stages.
func SumStages(stages []lib.Stage) (d lib.NullDuration) {
	for _, stage := range stages {
		d.Valid = stage.Duration.Valid
		if stage.Duration.Valid {
			d.Duration += stage.Duration.Duration
		}
	}
	return d
}
