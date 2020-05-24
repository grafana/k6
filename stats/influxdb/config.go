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

package influxdb

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/loadimpact/k6/lib/types"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	null "gopkg.in/guregu/null.v4"
)

type Config struct {
	// Connection.
	Addr             null.String        `json:"addr" envconfig:"K6_INFLUXDB_ADDR"`
	Username         null.String        `json:"username,omitempty" envconfig:"K6_INFLUXDB_USERNAME"`
	Password         null.String        `json:"password,omitempty" envconfig:"K6_INFLUXDB_PASSWORD"`
	Insecure         null.Bool          `json:"insecure,omitempty" envconfig:"K6_INFLUXDB_INSECURE"`
	PayloadSize      null.Int           `json:"payloadSize,omitempty" envconfig:"K6_INFLUXDB_PAYLOAD_SIZE"`
	PushInterval     types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_INFLUXDB_PUSH_INTERVAL"`
	ConcurrentWrites null.Int           `json:"concurrentWrites,omitempty" envconfig:"K6_INFLUXDB_CONCURRENT_WRITES"`

	// Samples.
	DB           null.String `json:"db" envconfig:"K6_INFLUXDB_DB"`
	Precision    null.String `json:"precision,omitempty" envconfig:"K6_INFLUXDB_PRECISION"`
	Retention    null.String `json:"retention,omitempty" envconfig:"K6_INFLUXDB_RETENTION"`
	Consistency  null.String `json:"consistency,omitempty" envconfig:"K6_INFLUXDB_CONSISTENCY"`
	TagsAsFields []string    `json:"tagsAsFields,omitempty" envconfig:"K6_INFLUXDB_TAGS_AS_FIELDS"`
}

func NewConfig() *Config {
	c := &Config{
		Addr:             null.NewString("http://localhost:8086", false),
		DB:               null.NewString("k6", false),
		TagsAsFields:     []string{"vu", "iter", "url"},
		ConcurrentWrites: null.NewInt(10, false),
		PushInterval:     types.NewNullDuration(time.Second, false),
	}
	return c
}

func (c Config) Apply(cfg Config) Config {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}
	if cfg.Username.Valid {
		c.Username = cfg.Username
	}
	if cfg.Password.Valid {
		c.Password = cfg.Password
	}
	if cfg.Insecure.Valid {
		c.Insecure = cfg.Insecure
	}
	if cfg.PayloadSize.Valid && cfg.PayloadSize.Int64 > 0 {
		c.PayloadSize = cfg.PayloadSize
	}
	if cfg.DB.Valid {
		c.DB = cfg.DB
	}
	if cfg.Precision.Valid {
		c.Precision = cfg.Precision
	}
	if cfg.Retention.Valid {
		c.Retention = cfg.Retention
	}
	if cfg.Consistency.Valid {
		c.Consistency = cfg.Consistency
	}
	if len(cfg.TagsAsFields) > 0 {
		c.TagsAsFields = cfg.TagsAsFields
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}

	if cfg.ConcurrentWrites.Valid {
		c.ConcurrentWrites = cfg.ConcurrentWrites
	}
	return c
}

// ParseArg parses an argument string into a Config
func ParseArg(arg string) (Config, error) {
	c := Config{}
	params, err := strvals.Parse(arg)

	if err != nil {
		return c, err
	}

	c, err = ParseMap(params)
	return c, err
}

// ParseMap parses a map[string]interface{} into a Config
func ParseMap(m map[string]interface{}) (Config, error) {
	c := Config{}
	if v, ok := m["tagsAsFields"].(string); ok {
		m["tagsAsFields"] = []string{v}
	}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: types.NullDecoder,
		Result:     &c,
	})
	if err != nil {
		return c, err
	}

	err = dec.Decode(m)
	return c, err
}

func ParseURL(text string) (Config, error) {
	c := Config{}
	u, err := url.Parse(text)
	if err != nil {
		return c, err
	}
	if u.Host != "" {
		c.Addr = null.StringFrom(u.Scheme + "://" + u.Host)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		c.DB = null.StringFrom(db)
	}
	if u.User != nil {
		c.Username = null.StringFrom(u.User.Username())
		pass, _ := u.User.Password()
		c.Password = null.StringFrom(pass)
	}
	for k, vs := range u.Query() {
		switch k {
		case "insecure":
			switch vs[0] {
			case "":
			case "false":
				c.Insecure = null.BoolFrom(false)
			case "true":
				c.Insecure = null.BoolFrom(true)
			default:
				return c, errors.Errorf("insecure must be true or false, not %s", vs[0])
			}
		case "payload_size":
			var size int
			size, err = strconv.Atoi(vs[0])
			if err != nil {
				return c, err
			}
			c.PayloadSize = null.IntFrom(int64(size))
		case "precision":
			c.Precision = null.StringFrom(vs[0])
		case "retention":
			c.Retention = null.StringFrom(vs[0])
		case "consistency":
			c.Consistency = null.StringFrom(vs[0])

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
		case "tagsAsFields":
			c.TagsAsFields = vs
		default:
			return c, errors.Errorf("unknown query parameter: %s", k)
		}
	}
	return c, err
}
