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

package cmd

import (
	"encoding/json"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/lib"
	"github.com/shibukawa/configdir"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"
)

const configFilename = "k6.json"

var (
	configDirs    = configdir.New("loadimpact", "k6")
	configFlagSet = pflag.NewFlagSet("", 0)
)

func init() {
	configFlagSet.SortFlags = false
	configFlagSet.StringP("out", "o", "", "`uri` for an external metrics database")
	configFlagSet.BoolP("linger", "l", false, "keep the API server alive past test end")
	configFlagSet.Bool("no-usage-report", false, "don't send anonymous stats to the developers")
}

type Config struct {
	lib.Options

	Out           null.String `json:"out" envconfig:"out"`
	Linger        null.Bool   `json:"linger" envconfig:"linger"`
	NoUsageReport null.Bool   `json:"noUsageReport" envconfig:"no_usage_report"`

	Collectors map[string]json.RawMessage `json:"collectors"`
}

// Gets configuration from CLI flags.
func getConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Options:       opts,
		Out:           getNullString(flags, "out"),
		Linger:        getNullBool(flags, "linger"),
		NoUsageReport: getNullBool(flags, "no-usage-report"),
	}, nil
}

// Reads a configuration file from disk.
func readDiskConfig() (Config, *configdir.Config, error) {
	cdir := configDirs.QueryFolderContainsFile(configFilename)
	if cdir == nil {
		return Config{}, configDirs.QueryFolders(configdir.Global)[0], nil
	}
	data, err := cdir.ReadFile(configFilename)
	if err != nil {
		return Config{}, cdir, err
	}
	var conf Config
	if err := json.Unmarshal(data, &conf); err != nil {
		return conf, cdir, err
	}
	return conf, cdir, nil
}

// Writes configuration back to disk.
func writeDiskConfig(cdir *configdir.Config, conf Config) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}
	return cdir.WriteFile(configFilename, data)
}

// Reads configuration variables from the environment.
func readEnvConfig() (conf Config, err error) {
	err = envconfig.Process("k6", &conf)
	return conf, err
}

func (c Config) Apply(cfg Config) Config {
	c.Options = c.Options.Apply(cfg.Options)
	if cfg.Linger.Valid {
		c.Linger = cfg.Linger
	}
	if cfg.NoUsageReport.Valid {
		c.NoUsageReport = cfg.NoUsageReport
	}
	if cfg.Out.Valid {
		c.Out = cfg.Out
	}
	return c
}

func (c Config) ConfigureCollector(t string, out interface{}) error {
	if data, ok := c.Collectors[t]; ok {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Config) SetCollectorConfig(t string, conf interface{}) error {
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	if c.Collectors == nil {
		c.Collectors = make(map[string]json.RawMessage)
	}
	c.Collectors[t] = data
	return nil
}
