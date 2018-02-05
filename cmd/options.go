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
	"fmt"
	"net"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

func optionFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.Int64P("vus", "u", 1, "number of virtual users")
	flags.Int64P("max", "m", 0, "max available virtual users")
	flags.DurationP("duration", "d", 0, "test duration limit")
	flags.Int64P("iterations", "i", 0, "script iteration limit")
	flags.StringSliceP("stage", "s", nil, "add a `stage`, as `[duration]:[target]`")
	flags.BoolP("paused", "p", false, "start the test in a paused state")
	flags.Int64("max-redirects", 10, "follow at most n redirects")
	flags.Int64("batch", 10, "max parallel batch reqs")
	flags.Int64("batch-per-host", 0, "max parallel batch reqs per host")
	flags.Int64("rps", 0, "limit requests per second")
	flags.String("user-agent", fmt.Sprintf("k6/%s (https://k6.io/);", Version), "user agent for http requests")
	flags.String("http-debug", "", "log all HTTP requests and responses. Excludes body by default. To include body use '---http-debug=full'")
	flags.Lookup("http-debug").NoOptDefVal = "headers"
	flags.Bool("insecure-skip-tls-verify", false, "skip verification of TLS certificates")
	flags.Bool("no-connection-reuse", false, "don't reuse connections between iterations")
	flags.BoolP("throw", "w", false, "throw warnings (like failed http requests) as errors")
	flags.StringSlice("blacklist-ip", nil, "blacklist an `ip range` from being called")
	flags.StringSlice("summary-trend-stats", nil, "define `stats` for trend metrics (response times), one or more as 'avg,p(95),...'")
	flags.StringSlice("default-tags", nil, "only include specified default tags on metrics")
	return flags
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
		RPS:                   getNullInt64(flags, "rps"),
		UserAgent:             getNullString(flags, "user-agent"),
		HttpDebug:             getNullString(flags, "http-debug"),
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

	trendStatStrings, err := flags.GetStringSlice("summary-trend-stats")
	if err != nil {
		return opts, err
	}
	for _, s := range trendStatStrings {
		if err := ui.VerifyTrendColumnStat(s); err != nil {
			return opts, errors.Wrapf(err, "stat '%s'", s)
		}

		opts.SummaryTrendStats = append(opts.SummaryTrendStats, s)
	}

	defTagsStrings, err := flags.GetStringSlice("default-tags")
	if err != nil {
		return opts, err
	}
	if len(defTagsStrings) > 0 {
		opts.DefaultTags = lib.Tags{}
		for _, tag := range defTagsStrings {
			opts.DefaultTags[tag] = true
		}
	}

	return opts, nil
}
