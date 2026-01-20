package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

// configFlagSet returns a FlagSet with the default run configuration flags.
func configFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.StringArrayP("out", "o", []string{}, "`uri` for an external metrics database")
	flags.BoolP("linger", "l", false, "keep the API server alive past test end")
	flags.Bool(
		"no-usage-report",
		false,
		"don't send anonymous usage"+"stats (https://grafana.com/docs/k6/latest/set-up/usage-collection/)",
	)
	return flags
}

// Config ...
type Config struct {
	lib.Options

	Out           []string  `json:"out" envconfig:"K6_OUT"`
	Linger        null.Bool `json:"linger" envconfig:"K6_LINGER"`
	NoUsageReport null.Bool `json:"noUsageReport" envconfig:"K6_NO_USAGE_REPORT"`
	WebDashboard  null.Bool `json:"webDashboard" envconfig:"K6_WEB_DASHBOARD"`

	// NoArchiveUpload is an option that is only used when running in local-execution mode with the cloud run
	// command.
	//
	// Because the implementation of k6 cloud run calls the same code as the k6 run command under the hood, we
	// need to be able to pass down a configuration option that is only relevant to the cloud run command.
	NoArchiveUpload null.Bool `json:"noArchiveUpload" envconfig:"K6_NO_ARCHIVE_UPLOAD"`

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
	if cfg.WebDashboard.Valid {
		c.WebDashboard = cfg.WebDashboard
	}
	if cfg.NoArchiveUpload.Valid {
		c.NoArchiveUpload = cfg.NoArchiveUpload
	}
	if len(cfg.Collectors) > 0 {
		c.Collectors = cfg.Collectors
	}
	return c
}

// getPartialConfig returns a Config but only parses the Options inside.
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

		// As the "run" and the "cloud run" commands share the same implementation
		// we enforce the run command to ignore the no-archive-upload flag, and always
		// set it to true (do not upload).
		NoArchiveUpload: null.NewBool(true, true),
	}, nil
}

// readDiskConfig reads the configuration file from the supplied filesystem and returns it or
// an error. The only situation in which an error won't be returned is if the
// user didn't explicitly specify a config file path and the default config file
// doesn't exist.
func readDiskConfig(gs *state.GlobalState) (Config, error) {
	// Try to see if the file exists in the supplied filesystem
	if _, err := gs.FS.Stat(gs.Flags.ConfigFilePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) && gs.Flags.ConfigFilePath == gs.DefaultFlags.ConfigFilePath {
			// If the file doesn't exist, but it was the default config file (i.e. the user
			// didn't specify anything), silence the error
			err = nil
		}
		return Config{}, err
	}

	data, err := fsext.ReadFile(gs.FS, gs.Flags.ConfigFilePath)
	if err != nil {
		return Config{}, fmt.Errorf("couldn't load the configuration from %q: %w", gs.Flags.ConfigFilePath, err)
	}
	var conf Config
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, fmt.Errorf("couldn't parse the configuration from %q: %w", gs.Flags.ConfigFilePath, err)
	}
	return conf, nil
}

// legacyConfigFilePath returns the path of the old location,
// which is now deprecated and superseded by a new default.
func legacyConfigFilePath(gs *state.GlobalState) string {
	return filepath.Join(gs.UserOSConfigDir, "loadimpact", "k6", "config.json")
}

// readLegacyDiskConfig reads the configuration file stored on the old default path.
func readLegacyDiskConfig(gs *state.GlobalState) (Config, error) {
	// Check if the legacy config exists in the supplied filesystem
	legacyPath := legacyConfigFilePath(gs)
	if _, err := gs.FS.Stat(legacyPath); err != nil {
		return Config{}, err
	}
	data, err := fsext.ReadFile(gs.FS, legacyPath)
	if err != nil {
		return Config{}, fmt.Errorf("couldn't load the configuration from %q: %w", legacyPath, err)
	}
	var conf Config
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, fmt.Errorf("couldn't parse the configuration from %q: %w", legacyPath, err)
	}
	return conf, nil
}

// writeDiskConfig serializes the configuration to a JSON file and writes it in the supplied
// location on the supplied filesystem.
func writeDiskConfig(gs *state.GlobalState, conf Config) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	if err := gs.FS.MkdirAll(filepath.Dir(gs.Flags.ConfigFilePath), 0o755); err != nil {
		return err
	}

	return fsext.WriteFile(gs.FS, gs.Flags.ConfigFilePath, data, 0o644)
}

// readEnvConfig reads configuration variables from the environment.
func readEnvConfig(envMap map[string]string) (Config, error) {
	// TODO: replace envconfig and refactor the whole configuration from the ground up :/
	conf := Config{}
	err := envconfig.Process("", &conf, func(key string) (string, bool) {
		v, ok := envMap[key]
		return v, ok
	})
	return conf, err
}

// loadConfigFile wraps the ordinary readDiskConfig operation.
// It adds the capability to fallbacks on the legacy default path if required.
//
// Unfortunately, readDiskConfig() silences the NotFound error.
// We don't want to change it as it is used across several places;
// and, hopefully, this code will be available only for a single major version.
// After we should restore to lookup only in a single location for config file (the default).
func loadConfigFile(gs *state.GlobalState) (Config, error) {
	// use directly the main flow if the user passed a custom path
	if gs.Flags.ConfigFilePath != gs.DefaultFlags.ConfigFilePath {
		return readDiskConfig(gs)
	}

	_, err := gs.FS.Stat(gs.Flags.ConfigFilePath)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		// if the passed path (the default) does not exist
		// then we attempt to load the legacy path
		legacyConf, legacyErr := readLegacyDiskConfig(gs)
		if legacyErr != nil && !errors.Is(legacyErr, fs.ErrNotExist) {
			return Config{}, legacyErr
		}
		// a legacy file has been found
		if legacyErr == nil {
			gs.Logger.Warnf("The configuration file has been found on the old default path (%q). "+
				"Please, run again `k6 cloud login` or `k6 login` commands to migrate to the new default path.\n\n",
				legacyConfigFilePath(gs))
			return legacyConf, nil
		}
		// the legacy file doesn't exist, then we fallback on the main flow
		// to return the silenced error for not existing config file
	}
	return readDiskConfig(gs)
}

// getConsolidatedConfig assemble the final consolidated configuration from all of the different sources:
// - start with the CLI-provided options to get shadowed (non-Valid) defaults in there
// - add the global file config options
// - add the Runner-provided options (they may come from Bundle too if applicable)
// - add the environment variables
// - merge the user-supplied CLI flags back in on top, to give them the greatest priority
// - set some defaults if they weren't previously specified
// TODO: add better validation, more explicit default values and improve consistency between formats
// TODO: accumulate all errors and differentiate between the layers?
func getConsolidatedConfig(gs *state.GlobalState, cliConf Config, runnerOpts lib.Options) (Config, error) {
	fileConf, err := loadConfigFile(gs)
	if err != nil {
		err = fmt.Errorf("failed to load the configuration file from the local file system: %w", err)
		return Config{}, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
	}

	envConf, err := readEnvConfig(gs.Env)
	if err != nil {
		return Config{}, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
	}

	conf := cliConf.Apply(fileConf)

	warnOnShortHandOverride(conf.Options, runnerOpts, "script", gs.Logger)
	conf = conf.Apply(Config{Options: runnerOpts})

	warnOnShortHandOverride(conf.Options, envConf.Options, "env", gs.Logger)
	conf = conf.Apply(envConf)

	warnOnShortHandOverride(conf.Options, cliConf.Options, "cli", gs.Logger)
	conf = conf.Apply(cliConf)

	conf = applyDefault(conf)

	// TODO(imiric): Move this validation where it makes sense in the configuration
	// refactor of #883. This repeats the trend stats validation already done
	// for CLI flags in cmd.getOptions, in case other configuration sources
	// (e.g. env vars) overrode our default value. This is not done in
	// lib.Options.Validate to avoid circular imports.
	if _, err = metrics.GetResolversForTrendColumns(conf.SummaryTrendStats); err != nil {
		return Config{}, err
	}

	return conf, nil
}

func warnOnShortHandOverride(a, b lib.Options, bName string, logger logrus.FieldLogger) {
	if a.Scenarios != nil &&
		(b.Duration.Valid || b.Iterations.Valid || b.Stages != nil || b.Scenarios != nil) {
		logger.Warnf(
			"%q level configuration overrode scenarios configuration entirely",
			bName)
	}
}

// applyDefault applies the default options value if it is not specified.
// This happens with types which are not supported by "gopkg.in/guregu/null.v3".
//
// Note that if you add option default value here, also add it in command line argument help text.
func applyDefault(conf Config) Config {
	if conf.SystemTags == nil {
		conf.SystemTags = &metrics.DefaultSystemTagSet
	}
	if conf.SummaryTrendStats == nil {
		conf.SummaryTrendStats = lib.DefaultSummaryTrendStats
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
	if !conf.SetupTimeout.Valid {
		conf.SetupTimeout.Duration = types.Duration(60 * time.Second)
	}
	if !conf.TeardownTimeout.Valid {
		conf.TeardownTimeout.Duration = types.Duration(60 * time.Second)
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

// migrateLegacyConfigFileIfAny copies the configuration file from
// the old default `~/.config/loadimpact/...` folder
// to the new `~/.config/k6/...` default folder.
// If the old file is not found no error is returned.
// It keeps the old file as a backup.
func migrateLegacyConfigFileIfAny(gs *state.GlobalState) error {
	fn := func() error {
		legacyFpath := legacyConfigFilePath(gs)
		_, err := gs.FS.Stat(legacyFpath)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		newPath := gs.DefaultFlags.ConfigFilePath
		if err := gs.FS.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
		// copy the config file leaving the old available as a backup
		f, err := fsext.ReadFile(gs.FS, legacyFpath)
		if err != nil {
			return err
		}
		err = fsext.WriteFile(gs.FS, newPath, f, 0o644)
		if err != nil {
			return err
		}
		gs.Logger.Infof("Note, the configuration file has been migrated "+
			"from the old default path (%q) to the new one (%q). "+
			"Clean up the old path after you verified that you can run tests by using the new configuration.\n\n",
			legacyFpath, newPath)
		return nil
	}
	if err := fn(); err != nil {
		return fmt.Errorf("move from the old to the new configuration's filepath failed: %w", err)
	}
	return nil
}
