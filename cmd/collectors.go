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
	"encoding"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/influxdb"
	jsonc "github.com/loadimpact/k6/stats/json"
	"github.com/loadimpact/k6/stats/statsd"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	collectorInfluxDB  = "influxdb"
	collectorJSON      = "json"
	collectorCloud     = "cloud"
	collectorStatsD    = "statsd"
	collectorDogStatsD = "dogstatsd"
)

func parseCollector(s string) (t, arg string) {
	parts := strings.SplitN(s, "=", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}

func newCollector(t, arg string, src *lib.SourceData, conf Config) (lib.Collector, error) {
	loadConfig := func(out encoding.TextUnmarshaler) error {
		if err := envconfig.Process("k6", out); err != nil {
			return err
		}
		if err := out.UnmarshalText([]byte(arg)); err != nil {
			return err
		}
		return nil
	}

	switch t {
	case collectorJSON:
		return jsonc.New(afero.NewOsFs(), arg)
	case collectorInfluxDB:
		config := conf.Collectors.InfluxDB
		if err := loadConfig(&config); err != nil {
			return nil, err
		}
		return influxdb.New(config)
	case collectorCloud:
		config := conf.Collectors.Cloud
		if err := loadConfig(&config); err != nil {
			return nil, err
		}
		return cloud.New(config, src, conf.Options, Version)
	case collectorStatsD:
		config := conf.Collectors.StatsD
		if err := loadConfig(&config); err != nil {
			return nil, err
		}
		return statsd.NewStatsD(config)
	case collectorDogStatsD:
		config := conf.Collectors.StatsD
		if err := loadConfig(&config); err != nil {
			return nil, err
		}
		return statsd.NewDogStatsD(config)
	default:
		return nil, errors.Errorf("unknown output type: %s", t)
	}
}
