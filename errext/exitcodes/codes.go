/*
 *
 * k6 - a next-generation load testing tool
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

// Package exitcodes contains the constants representing possible k6 exit error codes.
//nolint: golint
package exitcodes

import "go.k6.io/k6/errext"

const (
	CloudTestRunFailed       errext.ExitCode = 97 // This used to be 99 before k6 v0.33.0
	CloudFailedToGetProgress errext.ExitCode = 98
	ThresholdsHaveFailed     errext.ExitCode = 99
	SetupTimeout             errext.ExitCode = 100
	TeardownTimeout          errext.ExitCode = 101
	GenericTimeout           errext.ExitCode = 102 // TODO: remove?
	GenericEngine            errext.ExitCode = 103
	InvalidConfig            errext.ExitCode = 104
	ExternalAbort            errext.ExitCode = 105
	CannotStartRESTAPI       errext.ExitCode = 106
	ScriptException          errext.ExitCode = 107
	ScriptAborted            errext.ExitCode = 108
)
