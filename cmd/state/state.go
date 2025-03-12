package state

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sync"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/ui/console"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/secretsource"
)

const defaultConfigFileName = "config.json"

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

	env := BuildEnvMap(os.Environ())

	confDir, err := os.UserConfigDir()
	if err != nil {
		confDir = ".config"
	}

	binary, err := os.Executable()
	if err != nil {
		binary = "k6"
	}

	defaultFlags := GetDefaultFlags(confDir)
	flags := getFlags(defaultFlags, env, os.Args)

	logLevel := logrus.InfoLevel
	if flags.Verbose {
		logLevel = logrus.DebugLevel
	}

	logger := &logrus.Logger{
		Out: stderr,
		Formatter: &logrus.TextFormatter{
			ForceColors:   stderrTTY,
			DisableColors: !stderrTTY || flags.NoColor,
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
		Flags:           flags,
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
		Usage: usage.New(),
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

	BinaryProvisioning bool
	BuildServiceURL    string
}

// GetDefaultFlags returns the default global flags.
func GetDefaultFlags(homeDir string) GlobalFlags {
	return GlobalFlags{
		Address:          "localhost:6565",
		ProfilingEnabled: false,
		ConfigFilePath:   filepath.Join(homeDir, "k6", defaultConfigFileName),
		LogOutput:        "stderr",
		BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
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
	if env["K6_BINARY_PROVISIONING"] == "true" {
		result.BinaryProvisioning = true
	}
	if val, ok := env["K6_BUILD_SERVICE_URL"]; ok {
		result.BuildServiceURL = val
	}

	// check if verbose flag is set
	if slices.Contains(args, "-v") || slices.Contains(args, "--verbose") {
		result.Verbose = true
	}

	return result
}
