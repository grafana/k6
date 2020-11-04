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
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui"
)

var (
	ErrTagEmptyName   = errors.New("Invalid tag, empty name")
	ErrTagEmptyValue  = errors.New("Invalid tag, empty value")
	ErrTagEmptyString = errors.New("Invalid tag, empty string")
)

func optionFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.Int64P("vus", "u", 1, "number of virtual users")

	// TODO: delete in a few versions
	flags.Int64P("max", "m", 0, "max available virtual users")
	_ = flags.MarkDeprecated("max", "the global MaxVUs option is obsolete and doesn't affect the k6 script execution")

	flags.DurationP("duration", "d", 0, "test duration limit")
	flags.Int64P("iterations", "i", 0, "script total iteration limit (among all VUs)")
	flags.StringSliceP("stage", "s", nil, "add a `stage`, as `[duration]:[target]`")
	flags.String("execution-segment", "", "limit execution to the specified segment, e.g. 10%, 1/3, 0.2:2/3")
	flags.String("execution-segment-sequence", "", "the execution segment sequence") // TODO better description
	flags.BoolP("paused", "p", false, "start the test in a paused state")
	flags.Bool("no-setup", false, "don't run setup()")
	flags.Bool("no-teardown", false, "don't run teardown()")
	flags.Int64("max-redirects", 10, "follow at most n redirects")
	flags.Int64("batch", 20, "max parallel batch reqs")
	flags.Int64("batch-per-host", 6, "max parallel batch reqs per host")
	flags.Int64("rps", 0, "limit requests per second")
	flags.String("user-agent", fmt.Sprintf("k6/%s (https://k6.io/)", consts.Version), "user agent for http requests")
	flags.String("http-debug", "", "log all HTTP requests and responses. Excludes body by default. To include body use '--http-debug=full'")
	flags.Lookup("http-debug").NoOptDefVal = "headers"
	flags.Bool("insecure-skip-tls-verify", false, "skip verification of TLS certificates")
	flags.Bool("no-connection-reuse", false, "disable keep-alive connections")
	flags.Bool("no-vu-connection-reuse", false, "don't reuse connections between iterations")
	flags.Duration("min-iteration-duration", 0, "minimum amount of time k6 will take executing a single iteration")
	flags.BoolP("throw", "w", false, "throw warnings (like failed http requests) as errors")
	flags.StringSlice("blacklist-ip", nil, "blacklist an `ip range` from being called")
	flags.StringSlice("block-hostnames", nil, "block a case-insensitive hostname `pattern`,"+
		" with optional leading wildcard, from being called")

	// The comment about system-tags also applies for summary-trend-stats. The default values
	// are set in applyDefault().
	sumTrendStatsHelp := fmt.Sprintf(
		"define `stats` for trend metrics (response times), one or more as 'avg,p(95),...' (default '%s')",
		strings.Join(lib.DefaultSummaryTrendStats, ","),
	)
	flags.StringSlice("summary-trend-stats", nil, sumTrendStatsHelp)
	flags.String("summary-time-unit", "", "define the time unit used to display the trend stats. Possible units are: 's', 'ms' and 'us'")
	// system-tags must have a default value, but we can't specify it here, otherwiese, it will always override others.
	// set it to nil here, and add the default in applyDefault() instead.
	systemTagsCliHelpText := fmt.Sprintf(
		"only include these system tags in metrics (default %q)",
		stats.DefaultSystemTagSet.SetString(),
	)
	flags.StringSlice("system-tags", nil, systemTagsCliHelpText)
	flags.StringSlice("tag", nil, "add a `tag` to be applied to all samples, as `[name]=[value]`")
	flags.String("console-output", "", "redirects the console logging to the provided output file")
	flags.Bool("discard-response-bodies", false, "Read but don't process or save HTTP response bodies")
	flags.String("local-ips", "", "Client IP Ranges and/or CIDRs from which each VU will be making requests, "+
		"e.g. '192.168.220.1,192.168.0.10-192.168.0.25', 'fd:1::0/120', etc.")
	flags.String("dns", types.DefaultDNSConfig().String(), "DNS resolver configuration. Possible ttl values are: 'inf' "+
		"for a persistent cache, '0' to disable the cache,\nor a positive duration, e.g. '1s', '1m', etc. "+
		"Milliseconds are assumed if no unit is provided.\n"+
		"Possible select values to return a single IP are: 'first', 'random' or 'roundRobin'.\n"+
		"Possible policy values are: 'preferIPv4', 'preferIPv6', 'onlyIPv4', 'onlyIPv6' or 'any'.\n")
	return flags
}

func getOptions(flags *pflag.FlagSet) (lib.Options, error) {
	opts := lib.Options{
		VUs:                   getNullInt64(flags, "vus"),
		Duration:              getNullDuration(flags, "duration"),
		Iterations:            getNullInt64(flags, "iterations"),
		Paused:                getNullBool(flags, "paused"),
		NoSetup:               getNullBool(flags, "no-setup"),
		NoTeardown:            getNullBool(flags, "no-teardown"),
		MaxRedirects:          getNullInt64(flags, "max-redirects"),
		Batch:                 getNullInt64(flags, "batch"),
		BatchPerHost:          getNullInt64(flags, "batch-per-host"),
		RPS:                   getNullInt64(flags, "rps"),
		UserAgent:             getNullString(flags, "user-agent"),
		HTTPDebug:             getNullString(flags, "http-debug"),
		InsecureSkipTLSVerify: getNullBool(flags, "insecure-skip-tls-verify"),
		NoConnectionReuse:     getNullBool(flags, "no-connection-reuse"),
		NoVUConnectionReuse:   getNullBool(flags, "no-vu-connection-reuse"),
		MinIterationDuration:  getNullDuration(flags, "min-iteration-duration"),
		Throw:                 getNullBool(flags, "throw"),
		DiscardResponseBodies: getNullBool(flags, "discard-response-bodies"),
		// Default values for options without CLI flags:
		// TODO: find a saner and more dev-friendly and error-proof way to handle options
		SetupTimeout:    types.NullDuration{Duration: types.Duration(60 * time.Second), Valid: false},
		TeardownTimeout: types.NullDuration{Duration: types.Duration(60 * time.Second), Valid: false},

		MetricSamplesBufferSize: null.NewInt(1000, false),
	}

	// Using Changed() because GetStringSlice() doesn't differentiate between empty and no value
	if flags.Changed("stage") {
		stageStrings, err := flags.GetStringSlice("stage")
		if err != nil {
			return opts, err
		}
		opts.Stages = []lib.Stage{}
		for i, s := range stageStrings {
			var stage lib.Stage
			if err := stage.UnmarshalText([]byte(s)); err != nil {
				return opts, errors.Wrapf(err, "stage %d", i)
			}
			if !stage.Duration.Valid {
				return opts, fmt.Errorf("stage %d doesn't have a specified duration", i)
			}
			opts.Stages = append(opts.Stages, stage)
		}
	}

	if flags.Changed("execution-segment") {
		executionSegmentStr, err := flags.GetString("execution-segment")
		if err != nil {
			return opts, err
		}
		segment := new(lib.ExecutionSegment)
		err = segment.UnmarshalText([]byte(executionSegmentStr))
		if err != nil {
			return opts, err
		}
		opts.ExecutionSegment = segment
	}

	if flags.Changed("execution-segment-sequence") {
		executionSegmentSequenceStr, err := flags.GetString("execution-segment-sequence")
		if err != nil {
			return opts, err
		}
		segmentSequence := new(lib.ExecutionSegmentSequence)
		err = segmentSequence.UnmarshalText([]byte(executionSegmentSequenceStr))
		if err != nil {
			return opts, err
		}
		opts.ExecutionSegmentSequence = segmentSequence
	}

	if flags.Changed("system-tags") {
		systemTagList, err := flags.GetStringSlice("system-tags")
		if err != nil {
			return opts, err
		}
		opts.SystemTags = stats.ToSystemTagSet(systemTagList)
	}

	blacklistIPStrings, err := flags.GetStringSlice("blacklist-ip")
	if err != nil {
		return opts, err
	}
	for _, s := range blacklistIPStrings {
		net, parseErr := lib.ParseCIDR(s)
		if parseErr != nil {
			return opts, errors.Wrap(parseErr, "blacklist-ip")
		}
		opts.BlacklistIPs = append(opts.BlacklistIPs, net)
	}

	blockedHostnameStrings, err := flags.GetStringSlice("block-hostnames")
	if err != nil {
		return opts, err
	}
	if flags.Changed("block-hostnames") {
		opts.BlockedHostnames, err = types.NewNullHostnameTrie(blockedHostnameStrings)
		if err != nil {
			return opts, err
		}
	}

	localIpsString, err := flags.GetString("local-ips")
	if err != nil {
		return opts, err
	}
	if flags.Changed("local-ips") {
		err = opts.LocalIPs.UnmarshalText([]byte(localIpsString))
		if err != nil {
			return opts, err
		}
	}

	if flags.Changed("summary-trend-stats") {
		trendStats, errSts := flags.GetStringSlice("summary-trend-stats")
		if errSts != nil {
			return opts, errSts
		}
		if errSts = ui.ValidateSummary(trendStats); err != nil {
			return opts, errSts
		}
		opts.SummaryTrendStats = trendStats
	}

	summaryTimeUnit, err := flags.GetString("summary-time-unit")
	if err != nil {
		return opts, err
	}
	if summaryTimeUnit != "" {
		if summaryTimeUnit != "s" && summaryTimeUnit != "ms" && summaryTimeUnit != "us" {
			return opts, errors.New("invalid summary time unit. Use: 's', 'ms' or 'us'")
		}
		opts.SummaryTimeUnit = null.StringFrom(summaryTimeUnit)
	}

	runTags, err := flags.GetStringSlice("tag")
	if err != nil {
		return opts, err
	}

	if len(runTags) > 0 {
		parsedRunTags := make(map[string]string, len(runTags))
		for i, s := range runTags {
			name, value, err := parseTagNameValue(s)
			if err != nil {
				return opts, errors.Wrapf(err, "tag %d", i)
			}
			parsedRunTags[name] = value
		}
		opts.RunTags = stats.IntoSampleTags(&parsedRunTags)
	}

	redirectConFile, err := flags.GetString("console-output")
	if err != nil {
		return opts, err
	}

	if redirectConFile != "" {
		opts.ConsoleOutput = null.StringFrom(redirectConFile)
	}

	if dns, err := flags.GetString("dns"); err != nil {
		return opts, err
	} else if dns != "" {
		if err := opts.DNS.UnmarshalText([]byte(dns)); err != nil {
			return opts, err
		}
	}

	return opts, nil
}

func parseTagNameValue(nv string) (string, string, error) {
	if nv == "" {
		return "", "", ErrTagEmptyString
	}

	idx := strings.IndexRune(nv, '=')

	switch idx {
	case 0:
		return "", "", ErrTagEmptyName
	case -1, len(nv) - 1:
		return "", "", ErrTagEmptyValue
	default:
		return nv[:idx], nv[idx+1:], nil
	}
}
