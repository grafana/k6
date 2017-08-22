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

package main

import (
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/ghodss/yaml"
	"github.com/shibukawa/configdir"
)

const configFilename = "config.yml"

type ConfigCollectors map[string]interface{}

func (env *ConfigCollectors) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	res := make(map[string]interface{}, len(raw))
	for t, data := range raw {
		c := collectorOfType(t)
		if c == nil {
			log.Debugf("Config for unknown collector '%s'; skipping...", t)
			continue
		}

		cconf := c.MakeConfig()
		if err := json.Unmarshal(data, cconf); err != nil {
			return err
		}
		res[t] = cconf
	}

	*env = res
	return nil
}

func (env ConfigCollectors) Get(t string) interface{} {
	conf, ok := env[t]
	if !ok {
		conf = collectorOfType(t).MakeConfig()
	}
	return conf
}

// Global application configuration.
type Config struct {
	// Collector-specific data placeholders.
	Collectors ConfigCollectors `json:"collectors,omitempty"`
}

func LoadConfig() (conf Config, err error) {
	conf.Collectors = make(ConfigCollectors)

	cdir := configdir.New("loadimpact", "k6")
	folders := cdir.QueryFolders(configdir.Global)
	data, err := folders[0].ReadFile(configFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return conf, nil
		}
		return conf, err
	}

	if err := yaml.Unmarshal(data, &conf); err != nil {
		return conf, err
	}
	return conf, nil
}

func (c Config) Store() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	cdir := configdir.New("loadimpact", "k6")
	folders := cdir.QueryFolders(configdir.Global)
	return folders[0].WriteFile(configFilename, data)
}
