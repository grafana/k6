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

package csv

import (
	"strings"
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/loadimpact/k6/lib/types"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/guregu/null.v3"
)

// Config is the config for the csv collector
type Config struct {
	// Samples.
	FileName     null.String        `json:"file_name" envconfig:"K6_CSV_FILENAME"`
	SaveInterval types.NullDuration `json:"save_interval" envconfig:"K6_CSV_SAVE_INTERVAL"`
}

// config is a duplicate of ConfigFields as we can not mapstructure.Decode into
// null types so we duplicate the struct with primitive types to Decode into
type config struct {
	FileName     string `json:"file_name" mapstructure:"file_name" envconfig:"K6_CSV_FILENAME"`
	SaveInterval string `json:"save_interval" mapstructure:"save_interval" envconfig:"K6_CSV_SAVE_INTERVAL"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		FileName:     null.StringFrom("file.csv"),
		SaveInterval: types.NullDurationFrom(1 * time.Second),
	}
}

// Apply merges two configs by overwriting properties in the old config
func (c Config) Apply(cfg Config) Config {
	if cfg.FileName.Valid {
		c.FileName = cfg.FileName
	}
	if cfg.SaveInterval.Valid {
		c.SaveInterval = cfg.SaveInterval
	}
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}

	if !strings.Contains(arg, "=") {
		c.FileName = null.StringFrom(arg)
		c.SaveInterval = types.NullDurationFrom(1 * time.Second)
		return c, nil
	}

	params, err := strvals.Parse(arg)

	if err != nil {
		return c, err
	}

	if v, ok := params["save_interval"].(string); ok {
		err = c.SaveInterval.UnmarshalText([]byte(v))
		if err != nil {
			return c, err
		}
	}

	var cfg config
	err = mapstructure.Decode(params, &cfg)
	if err != nil {
		return c, err
	}

	c.FileName = null.StringFrom(cfg.FileName)

	return c, nil
}
