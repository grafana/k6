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

package version

import "fmt"

// Version represents a sember - k6 uses semantic versioning (http://semver.org/)
type Version struct {
	Major uint
	Minor uint
	Patch uint
}

// Current represents the current version and can be shared across packages
var Current = Version{Major: 0, Minor: 18, Patch: 2}

// Full returns the full semantic version as a string major.minor.patch
func Full() string {
	return fmt.Sprintf("%d.%d.%d", Current.Major, Current.Minor, Current.Patch)
}
