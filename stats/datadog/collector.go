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

package datadog

import (
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
	"github.com/sirupsen/logrus"
)

type tagHandler stats.TagSet

func (t tagHandler) processTags(tags map[string]string) []string {
	var res []string
	for key, value := range tags {
		if value != "" && !t[key] {
			res = append(res, key+":"+value)
		}
	}
	return res
}

// Config defines the datadog configuration
type Config struct {
	common.Config

	TagBlacklist stats.TagSet `json:"tagBlacklist,omitempty" envconfig:"TAG_BLACKLIST"`
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {
	c.Config = c.Config.Apply(cfg.Config)

	if cfg.TagBlacklist != nil {
		c.TagBlacklist = cfg.TagBlacklist
	}

	return c
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Config:       common.NewConfig(),
		TagBlacklist: stats.TagSet{},
	}
}

// New creates a new statsd connector client
func New(logger logrus.FieldLogger, conf Config) (*common.Collector, error) {
	return &common.Collector{
		Config:      conf.Config,
		Type:        "datadog",
		ProcessTags: tagHandler(conf.TagBlacklist).processTags,
		Logger:      logger,
	}, nil
}
