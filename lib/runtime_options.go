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

import (
	"fmt"
	"strings"

	null "gopkg.in/guregu/null.v4"
)

// CompatibilityMode specifies the JS compatibility mode
// nolint:lll
//go:generate enumer -type=CompatibilityMode -transform=snake -trimprefix CompatibilityMode -output compatibility_mode_gen.go
type CompatibilityMode uint8

const (
	// CompatibilityModeExtended achieves ES6+ compatibility with Babel and core.js
	CompatibilityModeExtended CompatibilityMode = iota + 1
	// CompatibilityModeBase is standard goja ES5.1+
	CompatibilityModeBase
)

// RuntimeOptions are settings passed onto the goja JS runtime
type RuntimeOptions struct {
	// Whether to pass the actual system environment variables to the JS runtime
	IncludeSystemEnvVars null.Bool `json:"includeSystemEnvVars"`

	// JS compatibility mode: "extended" (Goja+Babel+core.js) or "base" (plain Goja)
	//
	// TODO: when we resolve https://github.com/loadimpact/k6/issues/883, we probably
	// should use the CompatibilityMode type directly... but by then, we'd need to have
	// some way of knowing if the value has been set by the user or if we're using the
	// default one, so we can handle `k6 run --compatibility-mode=base es6_extended_archive.tar`
	CompatibilityMode null.String `json:"compatibilityMode"`

	// Environment variables passed onto the runner
	Env map[string]string `json:"env"`
}

// ValidateCompatibilityMode checks if the provided val is a valid compatibility mode
func ValidateCompatibilityMode(val string) (cm CompatibilityMode, err error) {
	if val == "" {
		return CompatibilityModeExtended, nil
	}
	if cm, err = CompatibilityModeString(val); err != nil {
		var compatValues []string
		for _, v := range CompatibilityModeValues() {
			compatValues = append(compatValues, v.String())
		}
		err = fmt.Errorf(`invalid compatibility mode "%s". Use: "%s"`,
			val, strings.Join(compatValues, `", "`))
	}
	return
}
