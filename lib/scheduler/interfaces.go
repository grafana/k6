/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package scheduler

import "time"

// Config is an interface that should be implemented by all scheduler config types
type Config interface {
	GetBaseConfig() BaseConfig
	Validate() []error
	GetMaxVUs() int64
	GetMaxDuration() time.Duration // includes max timeouts, to allow us to share VUs between schedulers in the future
	//TODO: Split(percentages []float64) ([]Config, error)
	//TODO: String() method that could be used for priting descriptions of the currently running schedulers for the UI?
}
