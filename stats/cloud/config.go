/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package cloud

import (
	"encoding/json"
)

type ConfigFields struct {
	Token           string `json:"token" mapstructure:"token" envconfig:"CLOUD_TOKEN"`
	Name            string `json:"name" mapstructure:"name" envconfig:"CLOUD_NAME"`
	Host            string `json:"host" mapstructure:"host" envconfig:"CLOUD_HOST"`
	Compress        bool   `json:"compress" mapstructure:"compress" envconfig:"CLOUD_COMPRESS"`
	ProjectID       int    `json:"project_id" mapstructure:"project_id" envconfig:"CLOUD_PROJECT_ID"`
	DeprecatedToken string `envconfig:"K6CLOUD_TOKEN"`
}

type Config ConfigFields

func (c Config) Apply(cfg Config) Config {
	if cfg.Token != "" {
		c.Token = cfg.Token
	}
	if cfg.Name != "" {
		c.Name = cfg.Name
	}
	if cfg.Host != "" {
		c.Host = cfg.Host
	}
	if cfg.ProjectID != 0 {
		c.ProjectID = cfg.ProjectID
	}
	return c
}

func (c *Config) UnmarshalText(data []byte) error {
	if s := string(data); s != "" {
		c.Name = s
	}
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
