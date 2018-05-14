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
	"encoding/json"

	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
)

type ConfigFields struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"KAFKA_BROKERS"`

	// Samples.
	Topic  string `json:"topic" envconfig:"KAFKA_TOPIC"`
	Format string `json:"format" envconfig:"KAFKA_FORMAT"`
}

type Config ConfigFields

func (c Config) Apply(cfg Config) Config {
	if len(cfg.Brokers) > 0 {
		c.Brokers = cfg.Brokers
	}
	if cfg.Format != "" {
		c.Format = cfg.Format
	}
	if cfg.Topic != "" {
		c.Topic = cfg.Topic
	}
	return c
}

// UnmarshalText takes an arg string and converts it to a config
func (c *Config) UnmarshalText(data []byte) error {
	params, err := strvals.Parse(string(data))

	if err != nil {
		return err
	}

	input := map[string]interface{}{
		"brokers": params["brokers"],
		"topic":   params["topic"],
		"format":  params["format"],
	}

	var config Config
	err = mapstructure.Decode(input, &config)
	if err != nil {
		return err
	}

	c.Brokers = config.Brokers
	c.Topic = config.Topic
	c.Format = config.Format

	return nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	fields := ConfigFields(*c)
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*c = Config(fields)
	return nil
}

func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(ConfigFields(c))
}
