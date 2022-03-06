package cmd

import (
	"archive/tar"
	"bytes"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/loader"
)

const (
	testTypeJS      = "js"
	testTypeArchive = "archive"
)

type loadedTest struct {
	testPath        string // contains the raw string the user supplied
	source          *loader.SourceData
	fileSystems     map[string]afero.Fs
	runtimeOptions  lib.RuntimeOptions
	metricsRegistry *metrics.Registry
	builtInMetrics  *metrics.BuiltinMetrics
	initRunner      lib.Runner // TODO: rename to something more appropriate

	// Only set if cliConfigGetter is supplied to loadTest() or if
	// consolidateDeriveAndValidateConfig() is manually called.
	consolidatedConfig Config
	derivedConfig      Config
}

func loadTest(
	gs *globalState, cmd *cobra.Command, args []string,
	// supply this if you want the test config consolidated and validated
	cliConfigGetter func(flags *pflag.FlagSet) (Config, error), // TODO: obviate
) (*loadedTest, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("k6 needs at least one argument to load the test")
	}

	testPath := args[0]
	gs.logger.Debugf("Resolving and reading test '%s'...", testPath)
	src, fileSystems, err := readSource(gs, testPath)
	if err != nil {
		return nil, err
	}
	resolvedPath := src.URL.String()
	gs.logger.Debugf("'%s' resolved to '%s' and successfully loaded %d bytes!", testPath, resolvedPath, len(src.Data))

	gs.logger.Debugf("Gathering k6 runtime options...")
	runtimeOptions, err := getRuntimeOptions(cmd.Flags(), gs.envVars)
	if err != nil {
		return nil, err
	}

	registry := metrics.NewRegistry()
	test := &loadedTest{
		testPath:        testPath,
		source:          src,
		fileSystems:     fileSystems,
		runtimeOptions:  runtimeOptions,
		metricsRegistry: registry,
		builtInMetrics:  metrics.RegisterBuiltinMetrics(registry),
	}

	gs.logger.Debugf("Initializing k6 runner for '%s' (%s)...", testPath, resolvedPath)
	if err := test.initializeFirstRunner(gs); err != nil {
		return nil, fmt.Errorf("could not initialize '%s': %w", testPath, err)
	}
	gs.logger.Debug("Runner successfully initialized!")

	if cliConfigGetter != nil {
		if err := test.consolidateDeriveAndValidateConfig(gs, cmd, cliConfigGetter); err != nil {
			return nil, err
		}
	}

	return test, nil
}

func (lt *loadedTest) initializeFirstRunner(gs *globalState) error {
	testPath := lt.source.URL.String()
	logger := gs.logger.WithField("test_path", testPath)

	testType := gs.flags.testType
	if testType == "" {
		logger.Debug("Detecting test type for...")
		testType = detectTestType(lt.source.Data)
	}

	switch testType {
	case testTypeJS:
		logger.Debug("Trying to load as a JS test...")
		runner, err := js.New(
			gs.logger, lt.source, lt.fileSystems, lt.runtimeOptions, lt.builtInMetrics, lt.metricsRegistry,
		)
		// TODO: should we use common.UnwrapGojaInterruptedError() here?
		if err != nil {
			return fmt.Errorf("could not load JS test '%s': %w", testPath, err)
		}
		lt.initRunner = runner
		return nil

	case testTypeArchive:
		logger.Debug("Trying to load test as an archive bundle...")

		var arc *lib.Archive
		arc, err := lib.ReadArchive(bytes.NewReader(lt.source.Data))
		if err != nil {
			return fmt.Errorf("could not load test archive bundle '%s': %w", testPath, err)
		}
		logger.Debugf("Loaded test as an archive bundle with type '%s'!", arc.Type)

		switch arc.Type {
		case testTypeJS:
			logger.Debug("Evaluating JS from archive bundle...")
			lt.initRunner, err = js.NewFromArchive(gs.logger, arc, lt.runtimeOptions, lt.builtInMetrics, lt.metricsRegistry)
			if err != nil {
				return fmt.Errorf("could not load JS from test archive bundle '%s': %w", testPath, err)
			}
			return nil
		default:
			return fmt.Errorf("archive '%s' has an unsupported test type '%s'", testPath, arc.Type)
		}
	default:
		return fmt.Errorf("unknown or unspecified test type '%s' for '%s'", testType, testPath)
	}
}

// readSource is a small wrapper around loader.ReadSource returning
// result of the load and filesystems map
func readSource(globalState *globalState, filename string) (*loader.SourceData, map[string]afero.Fs, error) {
	pwd, err := globalState.getwd()
	if err != nil {
		return nil, nil, err
	}

	filesystems := loader.CreateFilesystems(globalState.fs)
	src, err := loader.ReadSource(globalState.logger, filename, pwd, filesystems, globalState.stdIn)
	return src, filesystems, err
}

func detectTestType(data []byte) string {
	if _, err := tar.NewReader(bytes.NewReader(data)).Next(); err == nil {
		return testTypeArchive
	}
	return testTypeJS
}

func (lt *loadedTest) consolidateDeriveAndValidateConfig(
	gs *globalState, cmd *cobra.Command,
	cliConfGetter func(flags *pflag.FlagSet) (Config, error), // TODO: obviate
) error {
	var cliConfig Config
	if cliConfGetter != nil {
		gs.logger.Debug("Parsing CLI flags...")
		var err error
		cliConfig, err = cliConfGetter(cmd.Flags())
		if err != nil {
			return err
		}
	}

	gs.logger.Debug("Consolidating config layers...")
	consolidatedConfig, err := getConsolidatedConfig(gs, cliConfig, lt.initRunner.GetOptions())
	if err != nil {
		return err
	}

	gs.logger.Debug("Parsing thresholds and validating config...")
	// Parse the thresholds, only if the --no-threshold flag is not set.
	// If parsing the threshold expressions failed, consider it as an
	// invalid configuration error.
	if !lt.runtimeOptions.NoThresholds.Bool {
		for _, thresholds := range consolidatedConfig.Options.Thresholds {
			err = thresholds.Parse()
			if err != nil {
				return errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
			}
		}
	}

	derivedConfig, err := deriveAndValidateConfig(consolidatedConfig, lt.initRunner.IsExecutable, gs.logger)
	if err != nil {
		return err
	}

	lt.consolidatedConfig = consolidatedConfig
	lt.derivedConfig = derivedConfig

	return nil
}
