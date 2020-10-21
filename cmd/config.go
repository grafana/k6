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
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/executor"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/csv"
	"github.com/loadimpact/k6/stats/datadog"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/kafka"
	"github.com/loadimpact/k6/stats/statsd"
	"github.com/loadimpact/k6/ui"
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
	flags.String(
		"summary-export",
		"",
		"output the end-of-test summary report to JSON file",
	)
	return flags
}

type Config struct {
	lib.Options

	Out           []string    `json:"out" envconfig:"K6_OUT"`
	Linger        null.Bool   `json:"linger" envconfig:"K6_LINGER"`
	NoUsageReport null.Bool   `json:"noUsageReport" envconfig:"K6_NO_USAGE_REPORT"`
	NoThresholds  null.Bool   `json:"noThresholds" envconfig:"K6_NO_THRESHOLDS"`
	NoSummary     null.Bool   `json:"noSummary" envconfig:"K6_NO_SUMMARY"`
	SummaryExport null.String `json:"summaryExport" envconfig:"K6_SUMMARY_EXPORT"`

	Collectors struct {
		InfluxDB influxdb.Config `json:"influxdb"`
		Kafka    kafka.Config    `json:"kafka"`
		Cloud    cloud.Config    `json:"cloud"`
		StatsD   statsd.Config   `json:"statsd"`
		Datadog  datadog.Config  `json:"datadog"`
		CSV      csv.Config      `json:"csv"`
	} `json:"collectors"`
}

// Validate checks if all of the specified options make sense
func (c Config) Validate() []error {
	errors := c.Options.Validate()
	//TODO: validate all of the other options... that we should have already been validating...
	//TODO: maybe integrate an external validation lib: https://github.com/avelino/awesome-go#validation

	return errors
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
	if cfg.SummaryExport.Valid {
		c.SummaryExport = cfg.SummaryExport
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
		SummaryExport: getNullString(flags, "summary-export"),
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
		envconfig.Process("", &conf),
		envconfig.Process("", &conf.Collectors.Cloud),
		envconfig.Process("", &conf.Collectors.InfluxDB),
		envconfig.Process("", &conf.Collectors.Kafka),
		envconfig.Process("k6_statsd", &conf.Collectors.StatsD),
		envconfig.Process("k6_datadog", &conf.Collectors.Datadog),
	} {
		return conf, err
	}
	return conf, nil
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
	cliConf.Collectors.StatsD = statsd.NewConfig().Apply(cliConf.Collectors.StatsD)
	cliConf.Collectors.Datadog = datadog.NewConfig().Apply(cliConf.Collectors.Datadog)

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

	// TODO(imiric): Move this validation where it makes sense in the configuration
	// refactor of #883. This repeats the trend stats validation already done
	// for CLI flags in cmd.getOptions, in case other configuration sources
	// (e.g. env vars) overrode our default value. This is not done in
	// lib.Options.Validate to avoid circular imports.
	if err = ui.ValidateSummary(conf.SummaryTrendStats); err != nil {
		return conf, err
	}

	return conf, nil
}

// applyDefault applies the default options value if it is not specified.
// This happens with types which are not supported by "gopkg.in/guregu/null.v3".
//
// Note that if you add option default value here, also add it in command line argument help text.
func applyDefault(conf Config) Config {
	if conf.Options.SystemTags == nil {
		conf.Options.SystemTags = &stats.DefaultSystemTagSet
	}
	if conf.Options.SummaryTrendStats == nil {
		conf.Options.SummaryTrendStats = lib.DefaultSummaryTrendStats
	}
	defDNS := types.DefaultDNSConfig()
	if !conf.DNS.TTL.Valid {
		conf.DNS.TTL = defDNS.TTL
	}
	if !conf.DNS.Select.Valid {
		conf.DNS.Select = defDNS.Select
	}
	if !conf.DNS.Policy.Valid {
		conf.DNS.Policy = defDNS.Policy
	}

	return conf
}

func deriveAndValidateConfig(conf Config, isExecutable func(string) bool) (result Config, err error) {
	result = conf
	result.Options, err = executor.DeriveScenariosFromShortcuts(conf.Options)
	if err != nil {
		return result, err
	}
	return result, validateConfig(result, isExecutable)
}

func validateConfig(conf Config, isExecutable func(string) bool) error {
	errList := conf.Validate()

	for _, ec := range conf.Scenarios {
		if err := validateScenarioConfig(ec, isExecutable); err != nil {
			errList = append(errList, err)
		}
	}

	if len(errList) == 0 {
		return nil
	}

	errMsgParts := []string{"There were problems with the specified script configuration:"}
	for _, err := range errList {
		errMsgParts = append(errMsgParts, fmt.Sprintf("\t- %s", err.Error()))
	}

	return errors.New(strings.Join(errMsgParts, "\n"))
}

func validateScenarioConfig(conf lib.ExecutorConfig, isExecutable func(string) bool) error {
	execFn := conf.GetExec()
	if !isExecutable(execFn) {
		return fmt.Errorf("executor %s: function '%s' not found in exports", conf.GetName(), execFn)
	}
	return nil
}
