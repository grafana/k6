/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

import null "gopkg.in/guregu/null.v3"

// RuntimeOptions are settings passed onto the goja JS runtime
type RuntimeOptions struct {
	// Whether to disable the passing of actual system environment variables to the JS runtime
	NoSystemEnvVars null.Bool `json:"noSystemEnvVars" envconfig:"no_system_env_vars"`

	// Environment variables passed onto the runner
	Env map[string]string `json:"env" envconfig:"env"`
}

// Apply overwrites the receiver RuntimeOptions' fields with any that are set
// on the argument struct and returns the receiver
func (o RuntimeOptions) Apply(opts RuntimeOptions) RuntimeOptions {
	if opts.NoSystemEnvVars.Valid {
		o.NoSystemEnvVars = opts.NoSystemEnvVars
	}
	if opts.Env != nil {
		o.Env = opts.Env
	}
	return o
}
