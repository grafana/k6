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

import "time"

const (
	// Defaults

	DefaultLocale       string        = "en-US"
	DefaultScreenWidth  int64         = 1280
	DefaultScreenHeight int64         = 720
	DefaultTimeout      time.Duration = 30 * time.Second

	// Life-cycle consts

	LifeCycleNetworkIdleTimeout time.Duration = 500 * time.Millisecond
)
