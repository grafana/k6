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

package kafka

import (
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/guregu/null.v4"
)

// Config is the config for the kafka collector
type Config struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"K6_KAFKA_BROKERS"`

	// Samples.
	Topic        null.String        `json:"topic" envconfig:"K6_KAFKA_TOPIC"`
	Format       null.String        `json:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval types.NullDuration `json:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`

	InfluxDBConfig influxdb.Config `json:"influxdb"`
}

// config is a duplicate of ConfigFields as we can not mapstructure.Decode into
// null types so we duplicate the struct with primitive types to Decode into
type config struct {
	Brokers      []string `json:"brokers" mapstructure:"brokers" envconfig:"K6_KAFKA_BROKERS"`
	Topic        string   `json:"topic" mapstructure:"topic" envconfig:"K6_KAFKA_TOPIC"`
	Format       string   `json:"format" mapstructure:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval string   `json:"push_interval" mapstructure:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`

	InfluxDBConfig influxdb.Config `json:"influxdb" mapstructure:"influxdb"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Format:       null.StringFrom("json"),
		PushInterval: types.NullDurationFrom(1 * time.Second),
	}
}

func (c Config) Apply(cfg Config) Config {
	if len(cfg.Brokers) > 0 {
		c.Brokers = cfg.Brokers
	}
	if cfg.Format.Valid {
		c.Format = cfg.Format
	}
	if cfg.Topic.Valid {
		c.Topic = cfg.Topic
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}
	params, err := strvals.Parse(arg)

	if err != nil {
		return c, err
	}

	if v, ok := params["brokers"].(string); ok {
		params["brokers"] = []string{v}
	}

	if v, ok := params["influxdb"].(map[string]interface{}); ok {
		influxConfig, err := influxdb.ParseMap(v)
		if err != nil {
			return c, err
		}
		c.InfluxDBConfig = influxConfig
	}
	delete(params, "influxdb")

	if v, ok := params["push_interval"].(string); ok {
		err := c.PushInterval.UnmarshalText([]byte(v))
		if err != nil {
			return c, err
		}
	}

	var cfg config
	err = mapstructure.Decode(params, &cfg)
	if err != nil {
		return c, err
	}

	c.Brokers = cfg.Brokers
	c.Topic = null.StringFrom(cfg.Topic)
	c.Format = null.StringFrom(cfg.Format)

	return c, nil
}
