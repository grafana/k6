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

package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
)

// TODO: move this whole file out of the cmd package? maybe when fixing
// https://github.com/loadimpact/k6/issues/883, since this code is fairly
// self-contained and easily testable now, without any global dependencies...

var userEnvVarName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func parseEnvKeyValue(kv string) (string, string) {
	if idx := strings.IndexRune(kv, '='); idx != -1 {
		return kv[:idx], kv[idx+1:]
	}
	return kv, ""
}

func buildEnvMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v := parseEnvKeyValue(kv)
		env[k] = v
	}
	return env
}

func runtimeOptionFlagSet(includeSysEnv bool) *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.Bool("include-system-env-vars", includeSysEnv, "pass the real system environment variables to the runtime")
	flags.String("compatibility-mode", "extended",
		`JavaScript compiler compatibility mode, "extended" or "base"
base: pure Golang JS VM supporting ES5.1+
extended: base + Babel with ES2015 preset + core.js v2,
          slower and memory consuming but with greater JS support
`)
	flags.StringArrayP("env", "e", nil, "add/override environment variable with `VAR=value`")
	flags.Bool("no-thresholds", false, "don't run thresholds")
	flags.Bool("no-summary", false, "don't show the summary at the end of the test")
	flags.String(
		"summary-export",
		"",
		"output the end-of-test summary report to JSON file",
	)
	return flags
}

func saveBoolFromEnv(env map[string]string, varName string, placeholder *null.Bool) error {
	strValue, ok := env[varName]
	if !ok {
		return nil
	}
	val, err := strconv.ParseBool(strValue)
	if err != nil {
		return fmt.Errorf("env var '%s' is not a valid boolean value: %w", varName, err)
	}
	// Only override if not explicitly set via the CLI flag
	if !placeholder.Valid {
		*placeholder = null.BoolFrom(val)
	}
	return nil
}

func getRuntimeOptions(flags *pflag.FlagSet, environment map[string]string) (lib.RuntimeOptions, error) {
	// TODO: refactor with composable helpers as a part of #883, to reduce copy-paste
	// TODO: get these options out of the JSON config file as well?
	opts := lib.RuntimeOptions{
		IncludeSystemEnvVars: getNullBool(flags, "include-system-env-vars"),
		CompatibilityMode:    getNullString(flags, "compatibility-mode"),
		NoThresholds:         getNullBool(flags, "no-thresholds"),
		NoSummary:            getNullBool(flags, "no-summary"),
		SummaryExport:        getNullString(flags, "summary-export"),
		Env:                  make(map[string]string),
	}

	if envVar, ok := environment["K6_COMPATIBILITY_MODE"]; ok {
		// Only override if not explicitly set via the CLI flag
		if !opts.CompatibilityMode.Valid {
			opts.CompatibilityMode = null.StringFrom(envVar)
		}
	}
	if _, err := lib.ValidateCompatibilityMode(opts.CompatibilityMode.String); err != nil {
		// some early validation
		return opts, err
	}

	if err := saveBoolFromEnv(environment, "K6_INCLUDE_SYSTEM_ENV_VARS", &opts.IncludeSystemEnvVars); err != nil {
		return opts, err
	}
	if err := saveBoolFromEnv(environment, "K6_NO_THRESHOLDS", &opts.NoThresholds); err != nil {
		return opts, err
	}
	if err := saveBoolFromEnv(environment, "K6_NO_SUMMARY", &opts.NoSummary); err != nil {
		return opts, err
	}

	if envVar, ok := environment["K6_SUMMARY_EXPORT"]; ok {
		if !opts.SummaryExport.Valid {
			opts.SummaryExport = null.StringFrom(envVar)
		}
	}

	if opts.IncludeSystemEnvVars.Bool { // If enabled, gather the actual system environment variables
		opts.Env = environment
	}

	// Set/overwrite environment variables with custom user-supplied values
	envVars, err := flags.GetStringArray("env")
	if err != nil {
		return opts, err
	}
	for _, kv := range envVars {
		k, v := parseEnvKeyValue(kv)
		// Allow only alphanumeric ASCII variable names for now
		if !userEnvVarName.MatchString(k) {
			return opts, fmt.Errorf("invalid environment variable name '%s'", k)
		}
		opts.Env[k] = v
	}

	return opts, nil
}
