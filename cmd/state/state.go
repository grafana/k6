package state

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"go.k6.io/k6/lib"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/ui/console"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/secretsource"
)

const (
	// AutoExtensionResolution defines the environment variable that enables using extensions natively
	AutoExtensionResolution = "K6_AUTO_EXTENSION_RESOLUTION"

	// DependenciesManifest defines the default values for dependency resolution
	DependenciesManifest = "K6_DEPENDENCIES_MANIFEST"

	// communityExtensionsCatalog defines the catalog for community extensions
	communityExtensionsCatalog = "oss"

	// defaultBuildServiceURL defines the URL to the default (grafana hosted) build service
	defaultBuildServiceURL = "https://ingest.k6.io/builder/api/v1"

	defaultConfigFileName = "config.json"
	defaultBinaryCacheDir = "builds"
)

// GlobalState contains the GlobalFlags and accessors for most of the global
// process-external state like CLI arguments, env vars, standard input, output
// and error, etc. In practice, most of it is normally accessed through the `os`
// package from the Go stdlib.
//
// We group them here so we can prevent direct access to them from the rest of
// the k6 codebase. This gives us the ability to mock them and have robust and
// easy-to-write integration-like tests to check the k6 end-to-end behavior in
// any simulated conditions.
//
// `NewGlobalState()` returns a globalState object with the real `os`
// parameters, while `NewGlobalTestState()` can be used in tests to create
// simulated environments.
type GlobalState struct {
	Ctx context.Context

	FS              fsext.Fs
	Getwd           func() (string, error)
	UserOSConfigDir string
	BinaryName      string
	CmdArgs         []string
	Env             map[string]string
	Events          *event.System

	DefaultFlags, Flags GlobalFlags

	OutMutex       *sync.Mutex
	Stdout, Stderr *console.Writer
	Stdin          io.Reader

	OSExit       func(int)
	SignalNotify func(chan<- os.Signal, ...os.Signal)
	SignalStop   func(chan<- os.Signal)

	Logger         *logrus.Logger //nolint:forbidigo //TODO:change to FieldLogger
	FallbackLogger logrus.FieldLogger

	SecretsManager *secretsource.Manager
	Usage          *usage.Usage
	TestStatus     *lib.TestStatus
}

// NewGlobalState returns a new GlobalState with the given ctx.
// Ideally, this should be the only function in the whole codebase where we use
// global variables and functions from the os package. Anywhere else, things
// like os.Stdout, os.Stderr, os.Stdin, os.Getenv(), etc. should be removed and
// the respective properties of globalState used instead.
//
//nolint:forbidigo
func NewGlobalState(ctx context.Context) *GlobalState {
	isDumbTerm := os.Getenv("TERM") == "dumb"
	stdoutTTY := !isDumbTerm && (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
	stderrTTY := !isDumbTerm && (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd()))
	outMutex := &sync.Mutex{}
	stdout := &console.Writer{
		RawOutFd: int(os.Stdout.Fd()),
		Mutex:    outMutex,
		Writer:   colorable.NewColorable(os.Stdout),
		IsTTY:    stdoutTTY,
	}
	stderr := &console.Writer{
		RawOutFd: int(os.Stderr.Fd()),
		Mutex:    outMutex,
		Writer:   colorable.NewColorable(os.Stderr),
		IsTTY:    stderrTTY,
	}

	confDir, err := os.UserConfigDir()
	if err != nil {
		confDir = ".config"
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = ".cache"
	}

	binary, err := os.Executable()
	if err != nil {
		binary = "k6"
	}

	env := BuildEnvMap(os.Environ())
	defaultFlags := GetDefaultFlags(confDir, cacheDir)
	globalFlags := getFlags(defaultFlags, env, os.Args)

	logLevel := logrus.InfoLevel
	if globalFlags.Verbose {
		logLevel = logrus.DebugLevel
	}

	logger := &logrus.Logger{
		Out: stderr,
		Formatter: &logrus.TextFormatter{
			ForceColors:   stderrTTY,
			DisableColors: !stderrTTY || globalFlags.NoColor,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logLevel,
	}

	return &GlobalState{
		Ctx:             ctx,
		FS:              fsext.NewOsFs(),
		Getwd:           os.Getwd,
		UserOSConfigDir: confDir,
		BinaryName:      filepath.Base(binary),
		CmdArgs:         os.Args,
		Env:             env,
		Events:          event.NewEventSystem(100, logger),
		DefaultFlags:    defaultFlags,
		Flags:           globalFlags,
		OutMutex:        outMutex,
		Stdout:          stdout,
		Stderr:          stderr,
		Stdin:           os.Stdin,
		OSExit:          os.Exit,
		SignalNotify:    signal.Notify,
		SignalStop:      signal.Stop,
		Logger:          logger,
		FallbackLogger: &logrus.Logger{ // we may modify the other one
			Out:       stderr,
			Formatter: new(logrus.TextFormatter), // no fancy formatting here
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.InfoLevel,
		},
		Usage:      usage.New(),
		TestStatus: lib.NewTestStatus(),
	}
}

// GlobalFlags contains global config values that apply for all k6 sub-commands.
type GlobalFlags struct {
	ConfigFilePath   string
	Quiet            bool
	NoColor          bool
	Address          string
	ProfilingEnabled bool
	LogOutput        string
	SecretSource     []string
	LogFormat        string
	Verbose          bool

	AutoExtensionResolution   bool
	BuildServiceURL           string
	BinaryCache               string
	EnableCommunityExtensions bool
	DependenciesManifest      string
}

// GetDefaultFlags returns the default global flags.
func GetDefaultFlags(homeDir string, cacheDir string) GlobalFlags {
	return GlobalFlags{
		Address:                   "localhost:6565",
		ProfilingEnabled:          false,
		ConfigFilePath:            filepath.Join(homeDir, "k6", defaultConfigFileName),
		LogOutput:                 "stderr",
		AutoExtensionResolution:   true,
		BuildServiceURL:           defaultBuildServiceURL,
		EnableCommunityExtensions: false,
		BinaryCache:               filepath.Join(cacheDir, "k6", defaultBinaryCacheDir),
	}
}

func getFlags(defaultFlags GlobalFlags, env map[string]string, args []string) GlobalFlags {
	result := defaultFlags

	// TODO: add env vars for the rest of the values (after adjusting
	// rootCmdPersistentFlagSet(), of course)

	if val, ok := env["K6_CONFIG"]; ok {
		result.ConfigFilePath = val
	}
	if val, ok := env["K6_LOG_OUTPUT"]; ok {
		result.LogOutput = val
	}
	if val, ok := env["K6_LOG_FORMAT"]; ok {
		result.LogFormat = val
	}
	if env["K6_NO_COLOR"] != "" {
		result.NoColor = true
	}
	// Support https://no-color.org/, even an empty value should disable the
	// color output from k6.
	if _, ok := env["NO_COLOR"]; ok {
		result.NoColor = true
	}
	if _, ok := env["K6_PROFILING_ENABLED"]; ok {
		result.ProfilingEnabled = true
	}
	//  old name for the K6_AUTO_EXTENSION_RESOLUTION feature flag
	//  maintained for backward compatibility to be removed in a future release
	if v, ok := env["K6_BINARY_PROVISIONING"]; ok {
		vb, err := strconv.ParseBool(v)
		if err == nil {
			result.AutoExtensionResolution = vb
		}
	}
	if v, ok := env["K6_AUTO_EXTENSION_RESOLUTION"]; ok {
		vb, err := strconv.ParseBool(v)
		if err == nil {
			result.AutoExtensionResolution = vb
		}
	}
	if val, ok := env["K6_BUILD_SERVICE_URL"]; ok {
		result.BuildServiceURL = val
	}
	if v, ok := env["K6_ENABLE_COMMUNITY_EXTENSIONS"]; ok {
		vb, err := strconv.ParseBool(v)
		if err == nil {
			result.EnableCommunityExtensions = vb
		}
	}
	if val, ok := env["K6_DEPENDENCIES_MANIFEST"]; ok {
		result.DependenciesManifest = val
	}

	// adjust BuildServiceURL if community extensions are enable
	// community extensions flag only takes effect if the default build service is used
	// for custom build service URLs it has no effect (because the /oss path may not be implemented)
	if result.EnableCommunityExtensions && result.BuildServiceURL == defaultBuildServiceURL {
		result.BuildServiceURL = fmt.Sprintf("%s/%s", defaultBuildServiceURL, communityExtensionsCatalog)
	}

	// check if verbose flag is set
	if slices.Contains(args, "-v") || slices.Contains(args, "--verbose") {
		result.Verbose = true
	}

	return result
}
