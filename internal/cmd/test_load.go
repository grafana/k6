package cmd

import (
	"archive/tar"
	"bytes"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
)

const (
	testTypeJS      = "js"
	testTypeArchive = "archive"
)

// loadedTest contains all of data, details and dependencies of a loaded
// k6 test, but without any config consolidation.
type loadedTest struct {
	sourceRootPath string // contains the raw string the user supplied
	pwd            string
	source         *loader.SourceData
	fs             fsext.Fs
	fileSystems    map[string]fsext.Fs
	preInitState   *lib.TestPreInitState
	initRunner     lib.Runner // TODO: rename to something more appropriate
	keyLogger      io.Closer
	moduleResolver *modules.ModuleResolver
}

func loadLocalTest(gs *state.GlobalState, cmd *cobra.Command, args []string) (*loadedTest, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("k6 needs at least one argument to load the test")
	}

	sourceRootPath := args[0]
	gs.Logger.Debugf("Resolving and reading test '%s'...", sourceRootPath)
	src, fileSystems, pwd, err := readSource(gs, sourceRootPath)
	if err != nil {
		return nil, err
	}
	resolvedPath := src.URL.String()
	gs.Logger.Debugf(
		"'%s' resolved to '%s' and successfully loaded %d bytes!",
		sourceRootPath, resolvedPath, len(src.Data),
	)

	gs.Logger.Debugf("Gathering k6 runtime options...")
	runtimeOptions, err := getRuntimeOptions(cmd.Flags(), gs.Env)
	if err != nil {
		return nil, err
	}

	if runtimeOptions.CompatibilityMode.String == lib.CompatibilityModeExperimentalEnhanced.String() {
		gs.Logger.Warnf("CompatibilityMode %[1]q is deprecated. Types are stripped by default for `.ts` files. "+
			"Please move to using %[2]q instead as %[1]q will be removed in the future",
			lib.CompatibilityModeExperimentalEnhanced.String(), lib.CompatibilityModeBase.String())
	}

	registry := metrics.NewRegistry()
	state := &lib.TestPreInitState{
		Logger:         gs.Logger,
		RuntimeOptions: runtimeOptions,
		Registry:       registry,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Events:         gs.Events,
		LookupEnv: func(key string) (string, bool) {
			val, ok := gs.Env[key]
			return val, ok
		},
		Usage:          gs.Usage,
		SecretsManager: gs.SecretsManager,
		TestStatus:     gs.TestStatus,
	}

	test := &loadedTest{
		pwd:            pwd,
		sourceRootPath: sourceRootPath,
		source:         src,
		fs:             gs.FS,
		fileSystems:    fileSystems,
		preInitState:   state,
	}

	gs.Logger.Debugf("Initializing k6 runner for '%s' (%s)...", sourceRootPath, resolvedPath)
	if err := test.initializeFirstRunner(gs); err != nil {
		return nil, fmt.Errorf("could not initialize '%s': %w", sourceRootPath, err)
	}
	gs.Logger.Debug("Runner successfully initialized!")
	return test, nil
}

func (lt *loadedTest) initializeFirstRunner(gs *state.GlobalState) error {
	testPath := lt.source.URL.String()
	logger := gs.Logger.WithField("test_path", testPath)

	testType := lt.preInitState.RuntimeOptions.TestType.String
	if testType == "" {
		logger.Debug("Detecting test type for...")
		testType = detectTestType(lt.source.Data)
	}

	if lt.preInitState.RuntimeOptions.KeyWriter.Valid {
		logger.Warnf("SSLKEYLOGFILE was specified, logging TLS connection keys to '%s'...",
			lt.preInitState.RuntimeOptions.KeyWriter.String)
		keylogFilename := lt.preInitState.RuntimeOptions.KeyWriter.String
		// if path is absolute - no point doing anything
		if !filepath.IsAbs(keylogFilename) {
			// filepath.Abs could be used but it will get the pwd from `os` package instead of what is in lt.pwd
			// this is against our general approach of not using `os` directly and makes testing harder
			keylogFilename = filepath.Join(lt.pwd, keylogFilename)
		}
		f, err := lt.fs.OpenFile(keylogFilename, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("couldn't get absolute path for keylog file: %w", err)
		}
		lt.keyLogger = f
		lt.preInitState.KeyLogger = &syncWriter{w: f}
	}
	switch testType {
	case testTypeJS:
		specifier := lt.source.URL.String()
		pwd := lt.source.URL.JoinPath("../")
		logger.Debug("Trying to load as a JS test...")
		moduleResolver := js.NewModuleResolver(pwd, lt.preInitState, lt.fileSystems)
		err := errext.WithExitCodeIfNone(
			moduleResolver.LoadMainModule(pwd, specifier, lt.source.Data),
			exitcodes.ScriptException)
		err = tryResolveModulesExtensions(err, moduleResolver.Imported(), logger, lt.fileSystems, lt.source, gs)
		if err != nil {
			return fmt.Errorf("could not load JS test '%s': %w", testPath, err)
		}
		runner, err := js.New(lt.preInitState, lt.source, lt.fileSystems, moduleResolver)
		// TODO: should we use common.UnwrapGojaInterruptedError() here?
		if err != nil {
			return fmt.Errorf("could not load JS test '%s': %w", testPath, err)
		}
		lt.initRunner = runner
		lt.moduleResolver = runner.Bundle.ModuleResolver
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
			specifier := arc.Filename
			pwd := arc.PwdURL
			moduleResolver := js.NewModuleResolver(pwd, lt.preInitState, arc.Filesystems)
			err := errext.WithExitCodeIfNone(
				moduleResolver.LoadMainModule(pwd, specifier, arc.Data),
				exitcodes.ScriptException)
			err = tryResolveModulesExtensions(err, moduleResolver.Imported(), logger, arc.Filesystems, lt.source, gs)
			if err != nil {
				return fmt.Errorf("could not load JS test '%s': %w", testPath, err)
			}
			runner, err := js.NewFromArchive(lt.preInitState, arc, moduleResolver)
			if err != nil {
				return fmt.Errorf("could not load JS from test archive bundle '%s': %w", testPath, err)
			}
			lt.initRunner = runner
			lt.moduleResolver = runner.Bundle.ModuleResolver
			return nil
		default:
			return fmt.Errorf("archive '%s' has an unsupported test type '%s'", testPath, arc.Type)
		}
	default:
		return fmt.Errorf("unknown or unspecified test type '%s' for '%s'", testType, testPath)
	}
}

func tryResolveModulesExtensions(
	originalError error, imports []string, logger logrus.FieldLogger,
	fileSystems map[string]fsext.Fs, source *loader.SourceData, gs *state.GlobalState,
) error {
	if !gs.Flags.AutoExtensionResolution {
		return originalError
	}

	deps, err := extractUnknownModules(originalError)
	if err != nil {
		return err
	}
	err = analyseUseContraints(imports, fileSystems, deps)
	if err != nil {
		return err
	}
	if len(deps) == 0 {
		return nil
	}
	if !isCustomBuildRequired(deps, build.Version, ext.GetAll()) {
		logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		return nil
	}

	if source.URL.Path == "/-" {
		gs.Stdin = bytes.NewBuffer(source.Data)
	}

	return binaryIsNotSatisfyingDependenciesError{
		deps: deps,
	}
}

func analyseUseContraints(imports []string, fileSystems map[string]fsext.Fs, deps dependencies) error {
	for _, imported := range imports {
		if strings.HasPrefix(imported, "k6") {
			continue
		}
		u, err := url.Parse(imported)
		if err != nil {
			panic(err)
		}
		// We always have URLs here with scheme and everything
		_, path, _ := strings.Cut(imported, "://")
		if u.Scheme == "https" {
			path = "/" + path
		}
		data, err := fsext.ReadFile(fileSystems[u.Scheme], path)
		if err != nil {
			panic(err)
		}
		err = processUseDirectives(imported, data, deps)
		if err != nil {
			panic(err)
		}
	}
	return nil
}

type dependencies map[string]*semver.Constraints

func (d dependencies) update(dep, constraintStr string) error {
	var constraint *semver.Constraints
	var err error
	if len(constraintStr) > 0 {
		constraint, err = semver.NewConstraint(constraintStr)
		if err != nil {
			return fmt.Errorf("unparsable constraint %q for %q", constraintStr, dep)
		}
	}
	// TODO: We could actually do constraint comparison here and get the more specific one
	oldConstraint, ok := d[dep]
	if !ok || oldConstraint == nil { // either nothing or it didn't have constraint
		d[dep] = constraint
		return nil
	}
	if constraint == oldConstraint || constraint == nil {
		return nil
	}
	return fmt.Errorf("already have constraint for %q, when parsing %q", dep, constraint)
}

func (d dependencies) String() string {
	var buf bytes.Buffer

	for idx, depName := range slices.Sorted(maps.Keys(d)) {
		if idx > 0 {
			_ = buf.WriteByte(';')
		}

		buf.WriteString(depName)
		constraint := d[depName]
		if constraint != nil {
			buf.WriteString(constraint.String())
		}
	}
	return buf.String()
}

func extractUnknownModules(err error) (map[string]*semver.Constraints, error) {
	deps := make(map[string]*semver.Constraints)
	if err == nil {
		return deps, nil
	}

	var u modules.UnknownModulesError

	if errors.As(err, &u) {
		for _, name := range u.List() {
			deps[name] = nil
		}
		return deps, nil
	}

	return nil, err
}

// TODO(@mstoykov) potentially figure out some less "exceptionl workflow" solution
type binaryIsNotSatisfyingDependenciesError struct {
	deps dependencies
}

func (r binaryIsNotSatisfyingDependenciesError) Error() string {
	return fmt.Sprintf("binary does not satisfy dependencies %q", r.deps)
}

// readSource is a small wrapper around loader.ReadSource returning
// result of the load and filesystems map
func readSource(gs *state.GlobalState, filename string) (*loader.SourceData, map[string]fsext.Fs, string, error) {
	pwd, err := gs.Getwd()
	if err != nil {
		return nil, nil, "", err
	}

	filesystems := loader.CreateFilesystems(gs.FS)
	src, err := loader.ReadSource(gs.Logger, filename, pwd, filesystems, gs.Stdin)
	return src, filesystems, pwd, err
}

func detectTestType(data []byte) string {
	if _, err := tar.NewReader(bytes.NewReader(data)).Next(); err == nil {
		return testTypeArchive
	}
	return testTypeJS
}

func (lt *loadedTest) consolidateDeriveAndValidateConfig(
	gs *state.GlobalState, cmd *cobra.Command,
	cliConfGetter func(flags *pflag.FlagSet) (Config, error), // TODO: obviate
) (*loadedAndConfiguredTest, error) {
	var cliConfig Config
	if cliConfGetter != nil {
		gs.Logger.Debug("Parsing CLI flags...")
		var err error
		cliConfig, err = cliConfGetter(cmd.Flags())
		if err != nil {
			return nil, err
		}
	}

	gs.Logger.Debug("Consolidating config layers...")
	consolidatedConfig, err := getConsolidatedConfig(gs, cliConfig, lt.initRunner.GetOptions())
	if err != nil {
		return nil, err
	}

	gs.Logger.Debug("Parsing thresholds and validating config...")
	// Parse the thresholds, only if the --no-threshold flag is not set.
	// If parsing the threshold expressions failed, consider it as an
	// invalid configuration error.
	if !lt.preInitState.RuntimeOptions.NoThresholds.Bool {
		for metricName, thresholdsDefinition := range consolidatedConfig.Thresholds {
			err = thresholdsDefinition.Parse()
			if err != nil {
				return nil, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
			}

			err = thresholdsDefinition.Validate(metricName, lt.preInitState.Registry)
			if err != nil {
				return nil, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
			}
		}
	}

	derivedConfig, err := deriveAndValidateConfig(consolidatedConfig, lt.initRunner.IsExecutable, gs.Logger)
	if err != nil {
		return nil, err
	}

	return &loadedAndConfiguredTest{
		loadedTest:         lt,
		consolidatedConfig: consolidatedConfig,
		derivedConfig:      derivedConfig,
	}, nil
}

// loadedAndConfiguredTest contains the whole loadedTest, as well as the
// consolidated test config and the full test run state.
type loadedAndConfiguredTest struct {
	*loadedTest
	consolidatedConfig Config
	derivedConfig      Config
}

func loadAndConfigureLocalTest(
	gs *state.GlobalState, cmd *cobra.Command, args []string,
	cliConfigGetter func(flags *pflag.FlagSet) (Config, error),
) (*loadedAndConfiguredTest, error) {
	test, err := loadLocalTest(gs, cmd, args)
	if err != nil {
		return nil, err
	}

	return test.consolidateDeriveAndValidateConfig(gs, cmd, cliConfigGetter)
}

// loadSystemCertPool attempts to load system certificates.
func loadSystemCertPool(logger logrus.FieldLogger) {
	if _, err := x509.SystemCertPool(); err != nil {
		logger.WithError(err).Warning("Unable to load system cert pool")
	}
}

func (lct *loadedAndConfiguredTest) buildTestRunState(
	configToReinject lib.Options,
) (*lib.TestRunState, error) {
	// This might be the full derived or just the consolidated options
	if err := lct.initRunner.SetOptions(configToReinject); err != nil {
		return nil, err
	}

	// Here, where we get the consolidated options, is where we check if any
	// of the deprecated options is being used, and we report it.
	if _, isPresent := configToReinject.External["loadimpact"]; isPresent {
		if err := lct.preInitState.Usage.Uint64("deprecations/options.ext.loadimpact", 1); err != nil {
			return nil, err
		}
	}

	// it pre-loads system certificates to avoid doing it on the first TLS request.
	// This is done async to avoid blocking the rest of the loading process as it will not stop if it fails.
	go loadSystemCertPool(lct.preInitState.Logger)

	return &lib.TestRunState{
		TestPreInitState: lct.preInitState,
		Runner:           lct.initRunner,
		Options:          lct.derivedConfig.Options, // we will always run with the derived options
		RunTags:          lct.preInitState.Registry.RootTagSet().WithTagsFromMap(configToReinject.RunTags),
		GroupSummary:     lib.NewGroupSummary(lct.preInitState.Logger),
	}, nil
}

type syncWriter struct {
	w io.Writer
	m sync.Mutex
}

func (cw *syncWriter) Write(b []byte) (int, error) {
	cw.m.Lock()
	defer cw.m.Unlock()
	return cw.w.Write(b)
}
