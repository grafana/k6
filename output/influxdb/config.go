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
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

type Config struct {
	// Connection.
	Addr                  null.String        `json:"addr" envconfig:"K6_INFLUXDB_ADDR"`
	Username              null.String        `json:"username,omitempty" envconfig:"K6_INFLUXDB_USERNAME"`
	Password              null.String        `json:"password,omitempty" envconfig:"K6_INFLUXDB_PASSWORD"`
	Organization          null.String        `json:"organization" envconfig:"K6_INFLUXDB_ORGANIZATION"`
	Bucket                null.String        `json:"bucket" envconfig:"K6_INFLUXDB_BUCKET"`
	Token                 null.String        `json:"token" envconfig:"K6_INFLUXDB_TOKEN"`
	InsecureSkipTLSVerify null.Bool          `json:"insecureSkipTLSVerify,omitempty" envconfig:"K6_INFLUXDB_INSECURE"`
	PushInterval          types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_INFLUXDB_PUSH_INTERVAL"`
	ConcurrentWrites      null.Int           `json:"concurrentWrites,omitempty" envconfig:"K6_INFLUXDB_CONCURRENT_WRITES"`

	// Samples.
	DB           null.String        `json:"db" envconfig:"K6_INFLUXDB_DB"`
	Precision    types.NullDuration `json:"precision,omitempty" envconfig:"K6_INFLUXDB_PRECISION"`
	Retention    types.NullDuration `json:"retention,omitempty" envconfig:"K6_INFLUXDB_RETENTION"`
	TagsAsFields []string           `json:"tagsAsFields,omitempty" envconfig:"K6_INFLUXDB_TAGS_AS_FIELDS"`
}

// NewConfig creates a new InfluxDB output config with some default values.
func NewConfig() Config {
	c := Config{
		Addr:             null.NewString("http://localhost:8086", false),
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
	if cfg.DB.Valid {
		c.DB = cfg.DB
	}
	if cfg.InsecureSkipTLSVerify.Valid {
		c.InsecureSkipTLSVerify = cfg.InsecureSkipTLSVerify
	}
	if cfg.Username.Valid {
		c.Username = cfg.Username
	}
	if cfg.Password.Valid {
		c.Password = cfg.Password
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
	if cfg.Precision.Valid {
		c.Precision = cfg.Precision
	}
	if cfg.Retention.Valid {
		c.Retention = cfg.Retention
	}
	if cfg.Organization.Valid {
		c.Organization = cfg.Organization
	}
	if cfg.Bucket.Valid {
		c.Bucket = cfg.Bucket
	}
	if cfg.Token.Valid {
		c.Token = cfg.Token
	}
	return c
}

// SetFromKeyVals sets fields passed from a key and the associated values.
// e.g An URL Query field.
//nolint:cyclop
func (c *Config) SetFromKeyVals(k string, vs []string) (err error) {
	switch k {
	case "insecureSkipTLSVerify":
		err = c.InsecureSkipTLSVerify.UnmarshalText([]byte(vs[0]))
		if err != nil {
			return fmt.Errorf("insecureSkipTLSVerify must be true or false, not %s", vs[0])
		}
	case "precision":
		var d time.Duration
		d, err = types.ParseExtendedDuration(vs[0])
		if err != nil {
			return err
		}
		c.Precision = types.NullDurationFrom(d)
	case "retention":
		var d time.Duration
		d, err = types.ParseExtendedDuration(vs[0])
		if err != nil {
			return err
		}
		c.Retention = types.NullDurationFrom(d)
	case "pushInterval":
		err = c.PushInterval.UnmarshalText([]byte(vs[0]))
		if err != nil {
			return err
		}
	case "concurrentWrites":
		var writes int
		writes, err = strconv.Atoi(vs[0])
		if err != nil {
			return err
		}
		c.ConcurrentWrites = null.IntFrom(int64(writes))
	case "tagsAsFields":
		c.TagsAsFields = vs
	default:
		return fmt.Errorf("unknown query parameter: %s", k)
	}
	return nil
}

// ParseJSON parses the supplied JSON into a Config.
func ParseJSON(data json.RawMessage) (Config, error) {
	conf := Config{}
	err := json.Unmarshal(data, &conf)
	return conf, err
}

// ParseURL parses the supplied URL into a Config.
func ParseURL(text string, logger logrus.FieldLogger) (Config, error) {
	c := Config{}
	u, err := url.Parse(text)
	if err != nil {
		return c, err
	}
	if u.Host != "" {
		c.Addr = null.StringFrom(u.Scheme + "://" + u.Host)
	}
	if u.User != nil {
		c.Username = null.StringFrom(u.User.Username())
		pass, _ := u.User.Password()
		c.Password = null.StringFrom(pass)
	}
	if bucket := strings.TrimPrefix(u.Path, "/"); bucket != "" {
		c.Bucket = null.StringFrom(bucket)
	}
	for k, vs := range u.Query() {
		if k == "insecure" {
			logger.Warnf(
				"%q option is deprecated and it will be removed in the next releases, please use %q instead.",
				"insecure",
				"insecureSkipTLSVerify",
			)
			k = "insecureSkipTLSVerify"
		}
		err = c.SetFromKeyVals(k, vs)
		if err != nil {
			return c, err
		}
	}
	return c, err
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + URL config values}, and returns the final result.
func GetConsolidatedConfig(
	jsonRawConf json.RawMessage, env map[string]string, url string, logger logrus.FieldLogger,
) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf, err := ParseJSON(jsonRawConf)
		if err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envConfig := Config{}
	if err := envconfig.Process("", &envConfig); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, err
	}
	result = result.Apply(envConfig)

	if url != "" {
		urlConf, err := ParseURL(url, logger)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
