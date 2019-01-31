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

package datadog

import (
	"sort"
	"strings"

	"github.com/loadimpact/k6/stats/statsd/common"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

type tagHandler sort.StringSlice

func (t tagHandler) filterTags(tags map[string]string, group string) []string {
	var res []string

	for key, value := range tags {
		if value != "" && t.contains(key) {
			res = append(res, key+":"+value)
		}
	}
	return append(res, "group:"+group)
}

func (t tagHandler) contains(key string) bool {
	var n = ((sort.StringSlice)(t)).Search(key)
	return n != len(t) && t[n] == key
}

// Config defines the datadog configuration
type Config struct {
	common.Config

	TagWhitelist null.String `json:"tag_whitelist,omitempty" envconfig:"tag_whitelist" default:"status, method"`
}

// Apply returns config with defaults applied
func (c Config) Apply(cfg Config) Config {
	c.Config.Apply(cfg.Config)

	if cfg.TagWhitelist.Valid {
		c.TagWhitelist = cfg.TagWhitelist
	}

	return c
}

// New creates a new statsd connector client
func New(conf Config) (*common.Collector, error) {
	cl, err := common.MakeClient(conf.Config, common.Datadog)
	if err != nil {
		return nil, err
	}

	var tagsWhitelist = sort.StringSlice(strings.Split(conf.TagWhitelist.String, ","))
	for index := range tagsWhitelist {
		tagsWhitelist[index] = strings.TrimSpace(tagsWhitelist[index])
	}
	tagsWhitelist.Sort()
	return &common.Collector{
		Client:     cl,
		Config:     conf.Config,
		Logger:     log.WithField("type", common.Datadog.String()),
		Type:       common.Datadog,
		FilterTags: tagHandler(tagsWhitelist).filterTags,
	}, nil
}
