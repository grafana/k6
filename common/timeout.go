/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

// TimeoutSettings holds information on timeout settings.
type TimeoutSettings struct {
	parent                   *TimeoutSettings
	defaultTimeout           *int64
	defaultNavigationTimeout *int64
}

// NewTimeoutSettings creates a new timeout settings object.
func NewTimeoutSettings(parent *TimeoutSettings) *TimeoutSettings {
	t := &TimeoutSettings{
		parent:                   parent,
		defaultTimeout:           nil,
		defaultNavigationTimeout: nil,
	}
	return t
}

func (t *TimeoutSettings) setDefaultTimeout(timeout int64) {
	t.defaultTimeout = &timeout
}

func (t *TimeoutSettings) setDefaultNavigationTimeout(timeout int64) {
	t.defaultNavigationTimeout = &timeout
}

func (t *TimeoutSettings) navigationTimeout() int64 {
	if t.defaultNavigationTimeout != nil {
		return *t.defaultNavigationTimeout
	}
	if t.defaultTimeout != nil {
		return *t.defaultTimeout
	}
	if t.parent != nil {
		return t.parent.navigationTimeout()
	}
	return int64(DefaultTimeout.Seconds())
}

func (t *TimeoutSettings) timeout() int64 {
	if t.defaultTimeout != nil {
		return *t.defaultTimeout
	}
	if t.parent != nil {
		return t.parent.timeout()
	}
	return int64(DefaultTimeout.Seconds())
}
