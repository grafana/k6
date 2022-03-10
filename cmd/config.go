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

	"github.com/mstoykov/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

// configFlagSet returns a FlagSet with the default run configuration flags.
func configFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.StringArrayP("out", "o", []string{}, "`uri` for an external metrics database")
	flags.BoolP("linger", "l", false, "keep the API server alive past test end")
	flags.Bool("no-usage-report", false, "don't send anonymous stats to the developers")
	return flags
}

// Config ...
type Config struct {
	lib.Options

	Out           []string  `json:"out" envconfig:"K6_OUT"`
	Linger        null.Bool `json:"linger" envconfig:"K6_LINGER"`
	NoUsageReport null.Bool `json:"noUsageReport" envconfig:"K6_NO_USAGE_REPORT"`

	// TODO: deprecate
	Collectors map[string]json.RawMessage `json:"collectors"`
}

// Validate checks if all of the specified options make sense
func (c Config) Validate() []error {
	errors := c.Options.Validate()
	// TODO: validate all of the other options... that we should have already been validating...
	// TODO: maybe integrate an external validation lib: https://github.com/avelino/awesome-go#validation

	return errors
}

// Apply the provided config on top of the current one, returning a new one. The provided config has priority.
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
	if len(cfg.Collectors) > 0 {
		c.Collectors = cfg.Collectors
	}
	return c
}

// Returns a Config but only parses the Options inside.
func getPartialConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}

	return Config{Options: opts}, nil
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
	}, nil
}

// Reads the configuration file from the supplied filesystem and returns it or
// an error. The only situation in which an error won't be returned is if the
// user didn't explicitly specify a config file path and the default config file
// doesn't exist.
func readDiskConfig(globalState *globalState) (Config, error) {
	// Try to see if the file exists in the supplied filesystem
	if _, err := globalState.fs.Stat(globalState.flags.configFilePath); err != nil {
		if os.IsNotExist(err) && globalState.flags.configFilePath == globalState.defaultFlags.configFilePath {
			// If the file doesn't exist, but it was the default config file (i.e. the user
			// didn't specify anything), silence the error
			err = nil
		}
		return Config{}, err
	}

	data, err := afero.ReadFile(globalState.fs, globalState.flags.configFilePath)
	if err != nil {
		return Config{}, err
	}
	var conf Config
	return conf, json.Unmarshal(data, &conf)
}

// Serializes the configuration to a JSON file and writes it in the supplied
// location on the supplied filesystem
func writeDiskConfig(globalState *globalState, conf Config) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	if err := globalState.fs.MkdirAll(filepath.Dir(globalState.flags.configFilePath), 0o755); err != nil {
		return err
	}

	return afero.WriteFile(globalState.fs, globalState.flags.configFilePath, data, 0o644)
}

// Reads configuration variables from the environment.
func readEnvConfig(envMap map[string]string) (Config, error) {
	// TODO: replace envconfig and refactor the whole configuration from the ground up :/
	conf := Config{}
	err := envconfig.Process("", &conf, func(key string) (string, bool) {
		v, ok := envMap[key]
		return v, ok
	})
	return conf, err
}

// Assemble the final consolidated configuration from all of the different sources:
// - start with the CLI-provided options to get shadowed (non-Valid) defaults in there
// - add the global file config options
// - add the Runner-provided options (they may come from Bundle too if applicable)
// - add the environment variables
// - merge the user-supplied CLI flags back in on top, to give them the greatest priority
// - set some defaults if they weren't previously specified
// TODO: add better validation, more explicit default values and improve consistency between formats
// TODO: accumulate all errors and differentiate between the layers?
func getConsolidatedConfig(globalState *globalState, cliConf Config, runnerOpts lib.Options) (conf Config, err error) {
	// TODO: use errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig) where it makes sense?

	fileConf, err := readDiskConfig(globalState)
	if err != nil {
		return conf, err
	}
	envConf, err := readEnvConfig(globalState.envVars)
	if err != nil {
		return conf, err
	}

	conf = cliConf.Apply(fileConf)

	conf = conf.Apply(Config{Options: runnerOpts})

	conf = conf.Apply(envConf).Apply(cliConf)
	conf = applyDefault(conf)

	// TODO(imiric): Move this validation where it makes sense in the configuration
	// refactor of #883. This repeats the trend stats validation already done
	// for CLI flags in cmd.getOptions, in case other configuration sources
	// (e.g. env vars) overrode our default value. This is not done in
	// lib.Options.Validate to avoid circular imports.
	if _, err = metrics.GetResolversForTrendColumns(conf.SummaryTrendStats); err != nil {
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
		conf.Options.SystemTags = &metrics.DefaultSystemTagSet
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

func deriveAndValidateConfig(
	conf Config, isExecutable func(string) bool, logger logrus.FieldLogger,
) (result Config, err error) {
	result = conf
	result.Options, err = executor.DeriveScenariosFromShortcuts(conf.Options, logger)
	if err == nil {
		err = validateConfig(result, isExecutable)
	}
	return result, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
}

func validateConfig(conf Config, isExecutable func(string) bool) error {
	errList := conf.Validate()

	for _, ec := range conf.Scenarios {
		if err := validateScenarioConfig(ec, isExecutable); err != nil {
			errList = append(errList, err)
		}
	}

	return consolidateErrorMessage(errList, "There were problems with the specified script configuration:")
}

func consolidateErrorMessage(errList []error, title string) error {
	if len(errList) == 0 {
		return nil
	}

	errMsgParts := []string{title}
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
