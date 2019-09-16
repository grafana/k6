/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/scheduler"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/csv"
	"github.com/loadimpact/k6/stats/datadog"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/kafka"
	"github.com/loadimpact/k6/stats/statsd/common"
)

// configFlagSet returns a FlagSet with the default run configuration flags.
func configFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.StringArrayP("out", "o", []string{}, "`uri` for an external metrics database")
	flags.BoolP("linger", "l", false, "keep the API server alive past test end")
	flags.Bool("no-usage-report", false, "don't send anonymous stats to the developers")
	flags.Bool("no-thresholds", false, "don't run thresholds")
	flags.Bool("no-summary", false, "don't show the summary at the end of the test")
	return flags
}

type Config struct {
	lib.Options

	Out           []string  `json:"out" envconfig:"out"`
	Linger        null.Bool `json:"linger" envconfig:"linger"`
	NoUsageReport null.Bool `json:"noUsageReport" envconfig:"no_usage_report"`
	NoThresholds  null.Bool `json:"noThresholds" envconfig:"no_thresholds"`
	NoSummary     null.Bool `json:"noSummary" envconfig:"no_summary"`

	Collectors struct {
		InfluxDB influxdb.Config `json:"influxdb"`
		Kafka    kafka.Config    `json:"kafka"`
		Cloud    cloud.Config    `json:"cloud"`
		StatsD   common.Config   `json:"statsd"`
		Datadog  datadog.Config  `json:"datadog"`
		CSV      csv.Config      `json:"csv"`
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
	if cfg.NoSummary.Valid {
		c.NoSummary = cfg.NoSummary
	}
	c.Collectors.InfluxDB = c.Collectors.InfluxDB.Apply(cfg.Collectors.InfluxDB)
	c.Collectors.Cloud = c.Collectors.Cloud.Apply(cfg.Collectors.Cloud)
	c.Collectors.Kafka = c.Collectors.Kafka.Apply(cfg.Collectors.Kafka)
	c.Collectors.StatsD = c.Collectors.StatsD.Apply(cfg.Collectors.StatsD)
	c.Collectors.Datadog = c.Collectors.Datadog.Apply(cfg.Collectors.Datadog)
	c.Collectors.CSV = c.Collectors.CSV.Apply(cfg.Collectors.CSV)
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
		NoSummary:     getNullBool(flags, "no-summary"),
	}, nil
}

// Reads the configuration file from the supplied filesystem and returns it and its path.
// It will first try to see if the user explicitly specified a custom config file and will
// try to read that. If there's a custom config specified and it couldn't be read or parsed,
// an error will be returned.
// If there's no custom config specified and no file exists in the default config path, it will
// return an empty config struct, the default config location and *no* error.
func readDiskConfig(fs afero.Fs) (Config, string, error) {
	realConfigFilePath := configFilePath
	if realConfigFilePath == "" {
		// The user didn't specify K6_CONFIG or --config, use the default path
		realConfigFilePath = defaultConfigFilePath
	}

	// Try to see if the file exists in the supplied filesystem
	if _, err := fs.Stat(realConfigFilePath); err != nil {
		if os.IsNotExist(err) && configFilePath == "" {
			// If the file doesn't exist, but it was the default config file (i.e. the user
			// didn't specify anything), silence the error
			err = nil
		}
		return Config{}, realConfigFilePath, err
	}

	data, err := afero.ReadFile(fs, realConfigFilePath)
	if err != nil {
		return Config{}, realConfigFilePath, err
	}
	var conf Config
	err = json.Unmarshal(data, &conf)
	return conf, realConfigFilePath, err
}

// Serializes the configuration to a JSON file and writes it in the supplied
// location on the supplied filesystem
func writeDiskConfig(fs afero.Fs, configPath string, conf Config) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	if err := fs.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	return afero.WriteFile(fs, configPath, data, 0644)
}

// Reads configuration variables from the environment.
func readEnvConfig() (conf Config, err error) {
	// TODO: replace envconfig and refactor the whole configuration from the groun up :/
	for _, err := range []error{
		envconfig.Process("k6", &conf),
		envconfig.Process("k6", &conf.Collectors.Cloud),
		envconfig.Process("k6", &conf.Collectors.InfluxDB),
		envconfig.Process("k6", &conf.Collectors.Kafka),
	} {
		return conf, err
	}
	return conf, nil
}

type executionConflictConfigError string

func (e executionConflictConfigError) Error() string {
	return string(e)
}

var _ error = executionConflictConfigError("")

func getConstantLoopingVUsExecution(duration types.NullDuration, vus null.Int) scheduler.ConfigMap {
	ds := scheduler.NewConstantLoopingVUsConfig(lib.DefaultSchedulerName)
	ds.VUs = vus
	ds.Duration = duration
	return scheduler.ConfigMap{lib.DefaultSchedulerName: ds}
}

func getVariableLoopingVUsExecution(stages []lib.Stage, startVUs null.Int) scheduler.ConfigMap {
	ds := scheduler.NewVariableLoopingVUsConfig(lib.DefaultSchedulerName)
	ds.StartVUs = startVUs
	for _, s := range stages {
		if s.Duration.Valid {
			ds.Stages = append(ds.Stages, scheduler.Stage{Duration: s.Duration, Target: s.Target})
		}
	}
	return scheduler.ConfigMap{lib.DefaultSchedulerName: ds}
}

func getSharedIterationsExecution(iterations null.Int, duration types.NullDuration, vus null.Int) scheduler.ConfigMap {
	ds := scheduler.NewSharedIterationsConfig(lib.DefaultSchedulerName)
	ds.VUs = vus
	ds.Iterations = iterations
	if duration.Valid {
		ds.MaxDuration = duration
	}
	return scheduler.ConfigMap{lib.DefaultSchedulerName: ds}
}

// This checks for conflicting options and turns any shortcut options (i.e. duration, iterations,
// stages) into the proper scheduler configuration
func deriveExecutionConfig(conf Config) (Config, error) {
	result := conf
	switch {
	case conf.Iterations.Valid:
		if len(conf.Stages) > 0 { // stages isn't nil (not set) and isn't explicitly set to empty
			//TODO: make this an executionConflictConfigError in the next version
			logrus.Warn("Specifying both iterations and stages is deprecated and won't be supported in the future k6 versions")
		}

		result.Execution = getSharedIterationsExecution(conf.Iterations, conf.Duration, conf.VUs)
		// TODO: maybe add a new flag that will be used as a shortcut to per-VU iterations?

	case conf.Duration.Valid:
		if len(conf.Stages) > 0 { // stages isn't nil (not set) and isn't explicitly set to empty
			//TODO: make this an executionConflictConfigError in the next version
			logrus.Warn("Specifying both duration and stages is deprecated and won't be supported in the future k6 versions")
		}

		if conf.Duration.Duration <= 0 {
			//TODO: make this an executionConflictConfigError in the next version
			msg := "Specifying infinite duration in this way is deprecated and won't be supported in the future k6 versions"
			logrus.Warn(msg)
		} else {
			result.Execution = getConstantLoopingVUsExecution(conf.Duration, conf.VUs)
		}

	case len(conf.Stages) > 0: // stages isn't nil (not set) and isn't explicitly set to empty
		result.Execution = getVariableLoopingVUsExecution(conf.Stages, conf.VUs)

	default:
		if conf.Execution != nil { // If someone set this, regardless if its empty
			//TODO: remove this warning in the next version
			logrus.Warn("The execution settings are not functional in this k6 release, they will be ignored")
		}

		if len(conf.Execution) == 0 { // If unset or set to empty
			// No execution parameters whatsoever were specified, so we'll create a per-VU iterations config
			// with 1 VU and 1 iteration. We're choosing the per-VU config, since that one could also
			// be executed both locally, and in the cloud.
			result.Execution = scheduler.ConfigMap{
				lib.DefaultSchedulerName: scheduler.NewPerVUIterationsConfig(lib.DefaultSchedulerName),
			}
		}
	}

	//TODO: validate the config; questions:
	// - separately validate the duration, iterations and stages for better error messages?
	// - or reuse the execution validation somehow, at the end? or something mixed?
	// - here or in getConsolidatedConfig() or somewhere else?

	return result, nil
}

// Assemble the final consolidated configuration from all of the different sources:
// - start with the CLI-provided options to get shadowed (non-Valid) defaults in there
// - add the global file config options
// - if supplied, add the Runner-provided options
// - add the environment variables
// - merge the user-supplied CLI flags back in on top, to give them the greatest priority
// - set some defaults if they weren't previously specified
// TODO: add better validation, more explicit default values and improve consistency between formats
// TODO: accumulate all errors and differentiate between the layers?
func getConsolidatedConfig(fs afero.Fs, cliConf Config, runner lib.Runner) (conf Config, err error) {
	cliConf.Collectors.InfluxDB = influxdb.NewConfig().Apply(cliConf.Collectors.InfluxDB)
	cliConf.Collectors.Cloud = cloud.NewConfig().Apply(cliConf.Collectors.Cloud)
	cliConf.Collectors.Kafka = kafka.NewConfig().Apply(cliConf.Collectors.Kafka)

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
	conf = applyDefault(conf)

	return conf, nil
}

// applyDefault applys default options value if it is not specified by any mechenisms. This happens with types
// which does not support by "gopkg.in/guregu/null.v3".
//
// Note that if you add option default value here, also add it in command line argument help text.
func applyDefault(conf Config) Config {
	if conf.Options.SystemTags == nil {
		conf = conf.Apply(Config{Options: lib.Options{SystemTags: stats.ToSystemTagSet(stats.DefaultSystemTagList)}})
	}
	return conf
}

func deriveAndValidateConfig(conf Config) (Config, error) {
	result, err := deriveExecutionConfig(conf)
	if err != nil {
		return result, err
	}
	return result, validateConfig(conf)
}

//TODO: remove ↓
//nolint:unparam
func validateConfig(conf Config) error {
	errList := conf.Validate()
	if len(errList) == 0 {
		return nil
	}

	errMsgParts := []string{"There were problems with the specified script configuration:"}
	for _, err := range errList {
		errMsgParts = append(errMsgParts, fmt.Sprintf("\t- %s", err.Error()))
	}
	errMsg := errors.New(strings.Join(errMsgParts, "\n"))

	//TODO: actually return the error here instead of warning, so k6 aborts on config validation errors
	logrus.Warn(errMsg)
	return nil
}
