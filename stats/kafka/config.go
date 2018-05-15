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
	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/guregu/null.v3"
)

type ConfigFields struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"KAFKA_BROKERS"`

	// Samples.
	Topic  null.String `json:"topic" envconfig:"KAFKA_TOPIC"`
	Format null.String `json:"format" envconfig:"KAFKA_FORMAT"`
}

// config is a duplicate of ConfigFields as we can not mapstructure.Decode into
// null types so we duplicate the struct with primitive types to Decode into
type config struct {
	Brokers []string `json:"brokers" mapstructure:"brokers" envconfig:"KAFKA_BROKERS"`
	Topic   string   `json:"topic" mapstructure:"topic" envconfig:"KAFKA_TOPIC"`
	Format  string   `json:"format" mapstructure:"format" envconfig:"KAFKA_FORMAT"`
}

type Config ConfigFields

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Format: null.StringFrom("json"),
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

	input := map[string]interface{}{
		"brokers": params["brokers"],
		"topic":   params["topic"],
		"format":  params["format"],
	}

	var cfg config
	err = mapstructure.Decode(input, &cfg)
	if err != nil {
		return c, err
	}

	c.Brokers = cfg.Brokers
	c.Topic = null.StringFrom(cfg.Topic)
	c.Format = null.StringFrom(cfg.Format)

	return c, nil
}
