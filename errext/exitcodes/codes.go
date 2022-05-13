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

// ExitCode is just a type representing a process exit code for k6
type ExitCode uint8

// list of exit codes used by k6
const (
	CloudTestRunFailed       ExitCode = 97 // This used to be 99 before k6 v0.33.0
	CloudFailedToGetProgress ExitCode = 98
	ThresholdsHaveFailed     ExitCode = 99
	SetupTimeout             ExitCode = 100
	TeardownTimeout          ExitCode = 101
	GenericTimeout           ExitCode = 102 // TODO: remove?
	GenericEngine            ExitCode = 103
	InvalidConfig            ExitCode = 104
	ExternalAbort            ExitCode = 105
	CannotStartRESTAPI       ExitCode = 106
	ScriptException          ExitCode = 107
	ScriptAborted            ExitCode = 108
)
