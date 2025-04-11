package state

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

	DefaultFlags, Flags GlobalOptions

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
// This is expected to be difficult to unit test, we cover them with end-to-end CLI tests.
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

	binary, err := os.Executable()
	if err != nil {
		binary = "k6"
	}

	env := BuildEnvMap(os.Environ())

	defaultGlobalOptions := GetDefaultGlobalOptions(confDir)
	globalOptions := consolidateGlobalFlags(defaultGlobalOptions, env)

	logger := &logrus.Logger{
		Out: stderr,
		Formatter: &logrus.TextFormatter{
			ForceColors:   stderrTTY,
			DisableColors: !stderrTTY || globalOptions.NoColor,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.InfoLevel,
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
		DefaultFlags:    defaultGlobalOptions,
		Flags:           globalOptions,
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
