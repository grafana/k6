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
	"io/ioutil"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/kafka"
	"github.com/shibukawa/configdir"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"
)

const configFilename = "config.json"

var configDirs = configdir.New("loadimpact", "k6")
var configFile = os.Getenv("K6_CONFIG") // overridden by `-c` flag!

// configFileFlagSet returns a FlagSet that contains flags needed for specifying a config file.
func configFileFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.StringVarP(&configFile, "config", "c", configFile, "specify config file to read")
	return flags
}

// configFlagSet returns a FlagSet with the default run configuration flags.
func configFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.StringArrayP("out", "o", []string{}, "`uri` for an external metrics database")
	flags.BoolP("linger", "l", false, "keep the API server alive past test end")
	flags.Bool("no-usage-report", false, "don't send anonymous stats to the developers")
	flags.Bool("no-thresholds", false, "don't run thresholds")
	flags.AddFlagSet(configFileFlagSet())
	return flags
}

type Config struct {
	lib.Options

	Out           []string  `json:"out" envconfig:"out"`
	Linger        null.Bool `json:"linger" envconfig:"linger"`
	NoUsageReport null.Bool `json:"noUsageReport" envconfig:"no_usage_report"`
	NoThresholds  null.Bool `json:"noThresholds" envconfig:"no_thresholds"`

	Collectors struct {
		InfluxDB influxdb.Config `json:"influxdb"`
		Kafka    kafka.Config    `json:"kafka"`
		Cloud    cloud.Config    `json:"cloud"`
	} `json:"collectors"`
}

func (c Config) Apply(cfg Config) Config {
	c.Options = c.Options.Apply(cfg.Options)
	if len(cfg.Out) > 0 {
		c.Out = cfg.Out
	}
	if cfg.Linger.Valid {
		c.Linger = cfg.Linger
	}
	if cfg.NoUsageReport.Valid {
		c.NoUsageReport = cfg.NoUsageReport
	}
	if cfg.NoThresholds.Valid {
		c.NoThresholds = cfg.NoThresholds
	}
	c.Collectors.InfluxDB = c.Collectors.InfluxDB.Apply(cfg.Collectors.InfluxDB)
	c.Collectors.Cloud = c.Collectors.Cloud.Apply(cfg.Collectors.Cloud)
	return c
}

// Gets configuration from CLI flags.
func getConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}
	out, err := flags.GetStringArray("out")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Options:       opts,
		Out:           out,
		Linger:        getNullBool(flags, "linger"),
		NoUsageReport: getNullBool(flags, "no-usage-report"),
		NoThresholds:  getNullBool(flags, "no-thresholds"),
	}, nil
}

// Reads a configuration file from disk.
func readDiskConfig(fs afero.Fs) (Config, *configdir.Config, error) {
	if configFile != "" {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			return Config{}, nil, err
		}
		var conf Config
		err = json.Unmarshal(data, &conf)
		return conf, nil, err
	}

	cdir := configDirs.QueryFolderContainsFile(configFilename)
	if cdir == nil {
		return Config{}, configDirs.QueryFolders(configdir.Global)[0], nil
	}
	data, err := cdir.ReadFile(configFilename)
	if err != nil {
		return Config{}, cdir, err
	}
	var conf Config
	err = json.Unmarshal(data, &conf)
	return conf, cdir, err
}

// Writes configuration back to disk.
func writeDiskConfig(fs afero.Fs, cdir *configdir.Config, conf Config) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}
	if configFile != "" {
		return afero.WriteFile(fs, configFilename, data, 0644)
	}
	return cdir.WriteFile(configFilename, data)
}

// Reads configuration variables from the environment.
func readEnvConfig() (conf Config, err error) {
	err = envconfig.Process("k6", &conf)
	return conf, err
}

// Assemble the final consolidated configuration from all of the different sources:
// - start with the CLI-provided options to get shadowed (non-Valid) defaults in there
// - add the global file config options
// - if supplied, add the Runner-provided options
// - add the environment variables
// - merge the user-supplied CLI flags back in on top, to give them the greatest priority
// - set some defaults if they weren't previously specified
// TODO: add better validation, improve consistency between formats
func getConsolidatedConfig(fs afero.Fs, flags *pflag.FlagSet, runner lib.Runner) (conf Config, err error) {
	cliConf, err := getConfig(flags)
	if err != nil {
		return conf, err
	}
	fileConf, _, err := readDiskConfig(fs)
	if err != nil {
		return conf, err
	}
	envConf, err := readEnvConfig()
	if err != nil {
		return conf, err
	}

	conf = cliConf.Apply(fileConf)
	if runner != nil {
		conf = conf.Apply(Config{Options: runner.GetOptions()})
	}
	conf = conf.Apply(envConf).Apply(cliConf)

	// If -m/--max isn't specified, figure out the max that should be needed.
	if !conf.VUsMax.Valid {
		conf.VUsMax = null.NewInt(conf.VUs.Int64, conf.VUs.Valid)
		for _, stage := range conf.Stages {
			if stage.Target.Valid && stage.Target.Int64 > conf.VUsMax.Int64 {
				conf.VUsMax = stage.Target
			}
		}
	}

	// If -d/--duration, -i/--iterations and -s/--stage are all unset, run to one iteration.
	if !conf.Duration.Valid && !conf.Iterations.Valid && conf.Stages == nil {
		conf.Iterations = null.IntFrom(1)
	}

	// If duration is explicitly set to 0, it means run forever.
	if conf.Duration.Valid && conf.Duration.Duration == 0 {
		conf.Duration = types.NullDuration{}
	}

	return conf, nil
}
