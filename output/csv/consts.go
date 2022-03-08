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

// Package csv custom enum types used in config
package csv

// TimeFormat custom enum type
type TimeFormat string

// valid defined values for TimeFormat
const (
	Unix    TimeFormat = "unix"
	RFC3399 TimeFormat = "rfc3399"
)

// IsValid validates TimeFormat
func (timeFormat TimeFormat) IsValid() bool {
	switch timeFormat {
	case Unix, RFC3399:
		return true
	}
	return false
}
