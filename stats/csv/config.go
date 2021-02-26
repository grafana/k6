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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/guregu/null.v3"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/lib/types"
)

// Config is the config for the csv collector
type Config struct {
	// Samples.
	FileName     null.String        `json:"file_name" envconfig:"K6_CSV_FILENAME"`
	SaveInterval types.NullDuration `json:"save_interval" envconfig:"K6_CSV_SAVE_INTERVAL"`
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

	pairs := strings.Split(arg, ",")
	for _, pair := range pairs {
		r := strings.SplitN(pair, "=", 2)
		if len(r) != 2 {
			return c, fmt.Errorf("couldn't parse %q as argument for csv output", arg)
		}
		switch r[0] {
		case "save_interval":
			err := c.SaveInterval.UnmarshalText([]byte(r[1]))
			if err != nil {
				return c, err
			}
		case "file_name":
			c.FileName = null.StringFrom(r[1])
		default:
			return c, fmt.Errorf("unknown key %q as argument for csv output", r[0])
		}
	}

	return c, nil
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + arg config values}, and returns the final result.
func GetConsolidatedConfig(jsonRawConf json.RawMessage, env map[string]string, arg string) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
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

	if arg != "" {
		urlConf, err := ParseArg(arg)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
