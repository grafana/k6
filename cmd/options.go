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

package cmd

import (
	"net"

	"github.com/loadimpact/k6/lib"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

var optionFlagSet = pflag.NewFlagSet("", 0)

func init() {
	optionFlagSet.SortFlags = false
	optionFlagSet.Int64P("vus", "u", 1, "number of virtual users")
	optionFlagSet.Int64P("max", "m", 0, "max available virtual users")
	optionFlagSet.DurationP("duration", "d", 0, "test duration limit")
	optionFlagSet.Int64P("iterations", "i", 0, "script iteration limit")
	optionFlagSet.StringSliceP("stage", "s", nil, "add a `stage`, as `[duration]:[target]`")
	optionFlagSet.BoolP("paused", "p", false, "start the test in a paused state")
	optionFlagSet.Int64("max-redirects", 10, "follow at most n redirects")
	optionFlagSet.Int64("batch", 10, "max parallel http.batch() requests; 0 = unlimited")
	optionFlagSet.String("user-agent", "", "user agent for http requests")
	optionFlagSet.Bool("insecure-skip-tls-verify", false, "skip verification of TLS certificates")
	optionFlagSet.Bool("no-connection-reuse", false, "don't reuse connections between iterations")
	optionFlagSet.BoolP("throw", "w", false, "throw warnings (like failed http requests) as errors")
	optionFlagSet.StringSlice("blacklist-ip", nil, "blacklist an `ip range` from being called")
}

func getOptions(flags *pflag.FlagSet) (lib.Options, error) {
	opts := lib.Options{
		VUs:                   getNullInt64(flags, "vus"),
		VUsMax:                getNullInt64(flags, "max"),
		Duration:              getNullDuration(flags, "duration"),
		Iterations:            getNullInt64(flags, "iterations"),
		Paused:                getNullBool(flags, "paused"),
		MaxRedirects:          getNullInt64(flags, "max-redirects"),
		Batch:                 getNullInt64(flags, "batch"),
		UserAgent:             getNullString(flags, "user-agent"),
		InsecureSkipTLSVerify: getNullBool(flags, "insecure-skip-tls-verify"),
		NoConnectionReuse:     getNullBool(flags, "no-connection-reuse"),
		Throw:                 getNullBool(flags, "throw"),
	}

	stageStrings, err := flags.GetStringSlice("stage")
	if err != nil {
		return opts, err
	}
	if len(stageStrings) > 0 {
		opts.Stages = make([]lib.Stage, len(stageStrings))
		for i, s := range stageStrings {
			var stage lib.Stage
			if err := stage.UnmarshalText([]byte(s)); err != nil {
				return opts, errors.Wrapf(err, "stage %d", i)
			}
			opts.Stages[i] = stage
		}
	}

	blacklistIPStrings, err := flags.GetStringSlice("blacklist-ip")
	if err != nil {
		return opts, err
	}
	for _, s := range blacklistIPStrings {
		_, net, err := net.ParseCIDR(s)
		if err != nil {
			return opts, errors.Wrap(err, "blacklist-ip")
		}
		opts.BlacklistIPs = append(opts.BlacklistIPs, net)
	}

	return opts, nil
}
