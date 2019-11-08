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

package timescaledb

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/loadimpact/k6/lib/types"
	"github.com/pkg/errors"
	null "gopkg.in/guregu/null.v3"
)

type Config struct {
	// Connection URL in the form specified in the libpq docs,
	// see https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING):
	// postgresql://[user[:password]@][netloc][:port][,...][/dbname][?param1=value1&...]
	URL              null.String        `json:"addr" envconfig:"K6_TIMESCALEDB_URL"`
	PushInterval     types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_TIMESCALEDB_PUSH_INTERVAL"`
	ConcurrentWrites null.Int           `json:"concurrentWrites,omitempty" envconfig:"K6_TIMESCALEDB_CONCURRENT_WRITES"`

	addr null.String
	db   null.String
}

func NewConfig() *Config {
	c := &Config{
		URL:              null.NewString("postgresql://localhost/k6", false),
		ConcurrentWrites: null.NewInt(10, false),
		PushInterval:     types.NewNullDuration(time.Second, false),
	}
	return c
}

func (c Config) Apply(cfg Config) Config {
	if cfg.URL.Valid {
		c.URL = cfg.URL
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.ConcurrentWrites.Valid {
		c.ConcurrentWrites = cfg.ConcurrentWrites
	}
	return c
}

func ParseURL(text string) (Config, error) {
	c := Config{}
	u, err := url.Parse(text)
	if err != nil {
		return c, err
	}
	c.URL = null.NewString(u.String(), true)
	if u.Host != "" {
		c.addr = null.StringFrom(u.Scheme + "://" + u.Host)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		c.db = null.StringFrom(db)
	}
	for k, vs := range u.Query() {
		switch k {
		case "pushInterval":
			err = c.PushInterval.UnmarshalText([]byte(vs[0]))
			if err != nil {
				return c, err
			}
		case "concurrentWrites":
			var writes int
			writes, err = strconv.Atoi(vs[0])
			if err != nil {
				return c, err
			}
			c.ConcurrentWrites = null.IntFrom(int64(writes))
		default:
			return c, errors.Errorf("unknown query parameter: %s", k)
		}
	}
	return c, err
}
