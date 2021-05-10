/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package statsd

import (
	"encoding/json"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"
)

// TODO delete this whole file

type datadogConfig struct {
	Addr         null.String        `json:"addr,omitempty" envconfig:"K6_DATADOG_ADDR"`
	BufferSize   null.Int           `json:"bufferSize,omitempty" envconfig:"K6_DATADOG_BUFFER_SIZE"`
	Namespace    null.String        `json:"namespace,omitempty" envconfig:"K6_DATADOG_NAMESPACE"`
	PushInterval types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_DATADOG_PUSH_INTERVAL"`
	TagBlacklist stats.TagSet       `json:"tagBlacklist,omitempty" envconfig:"K6_DATADOG_TAG_BLACKLIST"`
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c datadogConfig) Apply(cfg datadogConfig) datadogConfig {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}
	if cfg.BufferSize.Valid {
		c.BufferSize = cfg.BufferSize
	}
	if cfg.Namespace.Valid {
		c.Namespace = cfg.Namespace
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.TagBlacklist != nil {
		c.TagBlacklist = cfg.TagBlacklist
	}

	return c
}

// NewdatadogConfig creates a new datadogConfig instance with default values for some fields.
func newdatadogConfig() datadogConfig {
	return datadogConfig{
		Addr:         null.NewString("localhost:8125", false),
		BufferSize:   null.NewInt(20, false),
		Namespace:    null.NewString("k6.", false),
		PushInterval: types.NewNullDuration(1*time.Second, false),
		TagBlacklist: stats.TagSet{},
	}
}

// GetConsolidateddatadogConfig combines {default config values + JSON config +
// environment vars}, and returns the final result.
func getConsolidatedDatadogConfig(jsonRawConf json.RawMessage) (datadogConfig, error) {
	result := newdatadogConfig()
	if jsonRawConf != nil {
		jsonConf := datadogConfig{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envdatadogConfig := datadogConfig{}
	if err := envconfig.Process("", &envdatadogConfig); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, err
	}
	result = result.Apply(envdatadogConfig)

	return result, nil
}

// NewDatadog creates a new statsd connector client with tags enabled
// TODO delete this
func NewDatadog(params output.Params) (*Output, error) {
	conf, err := getConsolidatedDatadogConfig(params.JSONConfig)
	if err != nil {
		return nil, err
	}
	logger := params.Logger.WithFields(logrus.Fields{"output": "statsd"})
	statsdConfig := config{
		Addr:         conf.Addr,
		BufferSize:   conf.BufferSize,
		Namespace:    conf.Namespace,
		PushInterval: conf.PushInterval,
		TagBlocklist: conf.TagBlacklist,
		EnableTags:   null.NewBool(true, false),
	}

	return &Output{
		config: statsdConfig,
		logger: logger,
	}, nil
}
