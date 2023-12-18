// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"errors"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHost   = ""
	defaultPort   = 5665
	defaultPeriod = time.Second * 10
	defaultOpen   = false
	defaultExport = ""
	defaultRecord = ""
)

func defaultTags() []string { return []string{"group"} }

type options struct {
	Port   int
	Host   string
	Period time.Duration
	Open   bool
	Export string
	Record string
	Tags   []string
	TagsS  string
}

func envopts(env map[string]string) (*options, error) {
	opts := &options{
		Port:   defaultPort,
		Host:   defaultHost,
		Period: defaultPeriod,
		Open:   defaultOpen,
		Export: defaultExport,
		Record: defaultRecord,
		Tags:   defaultTags(),
		TagsS:  "",
	}

	if len(env) == 0 {
		return opts, nil
	}

	if v, ok := env[envPort]; ok {
		i, e := strconv.Atoi(v)
		if e != nil {
			return nil, e
		}

		opts.Port = i
	}

	if v, ok := env[envHost]; ok {
		opts.Host = v
	}

	if v, ok := env[envExport]; ok {
		opts.Export = v
	} else if v, ok := env[envReport]; ok {
		opts.Export = v
	}

	if v, ok := env[envRecord]; ok {
		opts.Record = v
	}

	if v, ok := env[envPeriod]; ok {
		d, e := time.ParseDuration(v)
		if e != nil {
			return nil, errInvalidDuration
		}

		opts.Period = d
	}

	if v, ok := env[envOpen]; ok && v == "true" {
		opts.Open = true
	}

	if v, ok := env[envTags]; ok {
		opts.Tags = strings.Split(v, ",")
	}

	return opts, nil
}

func getopts(query string, env map[string]string) (*options, error) {
	opts, err := envopts(env)
	if err != nil {
		return nil, err
	}

	if query == "" {
		return opts, nil
	}

	value, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	if v := value.Get(paramPort); len(v) != 0 {
		i, e := strconv.Atoi(v)
		if e != nil {
			return nil, e
		}

		opts.Port = i
	}

	if v := value.Get(paramHost); len(v) != 0 {
		opts.Host = v
	}

	if v := value.Get(paramExport); len(v) != 0 {
		opts.Export = v
	} else if v := value.Get(paramReport); len(v) != 0 {
		opts.Export = v
	}

	if v := value.Get(paramRecord); len(v) != 0 {
		opts.Record = v
	}

	if v := value.Get(paramPeriod); len(v) != 0 {
		d, e := time.ParseDuration(v)
		if e != nil {
			return nil, errInvalidDuration
		}

		opts.Period = d
	}

	if v := value[paramTag]; len(v) != 0 {
		opts.Tags = v
	}

	if value.Has(paramOpen) && (len(value.Get(paramOpen)) == 0 || value.Get(paramOpen) == "true") {
		opts.Open = true
	}

	if v := value.Get(paramTags); len(v) != 0 {
		opts.Tags = append(opts.Tags, strings.Split(v, ",")...)
	}

	return opts, err
}

func (opts *options) addr() string {
	if opts.Port < 0 {
		return ""
	}

	return net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
}

func (opts *options) url() string {
	if opts.Port < 0 {
		return ""
	}

	host := opts.Host
	if host == "" {
		host = "127.0.0.1"
	}

	return "http://" + net.JoinHostPort(host, strconv.Itoa(opts.Port))
}

// period adjusts period, limit points per test run to 'points'.
func (opts *options) period(duration time.Duration) time.Duration {
	if duration == 0 {
		return opts.Period
	}

	optimal := float64(duration) / float64(points)

	return time.Duration(math.Ceil(optimal/float64(opts.Period))) * opts.Period
}

/*
approx. 1MB max report size, 8 hours test run with 10sec event period.
*/
const points = 2880

var errInvalidDuration = errors.New("invalid duration")

const (
	envPrefix = "K6_WEB_DASHBOARD_"

	paramPort = "port"
	envPort   = envPrefix + "PORT"

	paramHost = "host"
	envHost   = envPrefix + "HOST"

	paramPeriod = "period"
	envPeriod   = envPrefix + "PERIOD"

	paramOpen = "open"
	envOpen   = envPrefix + "OPEN"

	paramReport = "report"
	envReport   = envPrefix + "REPORT"
	paramExport = "export"
	envExport   = envPrefix + "EXPORT"

	paramRecord = "record"
	envRecord   = envPrefix + "RECORD"

	paramTag  = "tag"
	paramTags = "tags"
	envTags   = envPrefix + "TAGS"
)
