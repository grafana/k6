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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/schema"
)

const (
	defaultHost   = ""
	defaultPort   = 5665
	defaultPeriod = time.Second * 10
	defaultOpen   = false
	defaultReport = ""
	defaultRecord = ""
)

func defaultTags() []string { return []string{"group"} }

type options struct {
	Port   int
	Host   string
	Period time.Duration
	Open   bool
	Report string
	Record string
	Tags   []string `schema:"tag"`
	TagsS  string   `schema:"tags"`
}

func getopts(query string) (opts *options, err error) { //nolint:nonamedreturns
	opts = &options{
		Port:   defaultPort,
		Host:   defaultHost,
		Period: defaultPeriod,
		Open:   defaultOpen,
		Report: defaultReport,
		Record: defaultRecord,
		Tags:   defaultTags(),
		TagsS:  "",
	}

	if query == "" {
		return opts, nil
	}

	value, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	decoder := schema.NewDecoder()

	decoder.IgnoreUnknownKeys(true)

	decoder.RegisterConverter(time.Second, func(s string) reflect.Value {
		v, eerr := time.ParseDuration(s)
		if eerr != nil {
			return reflect.ValueOf(err)
		}

		return reflect.ValueOf(v)
	})

	defer func() {
		if r := recover(); r != nil {
			err = errInvalidDuration
		}
	}()

	if e := decoder.Decode(opts, value); e != nil {
		err = e
	}

	if value.Has("open") && len(value.Get("open")) == 0 {
		opts.Open = true
	}

	if len(opts.TagsS) != 0 {
		opts.Tags = append(opts.Tags, strings.Split(opts.TagsS, ",")...)
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
