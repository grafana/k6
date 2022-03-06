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

// Package cmd the package implementing all of cli interface of k6
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	stdlog "log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/log"
)

const (
	defaultConfigFileName   = "config.json"
	waitRemoteLoggerTimeout = time.Second * 5
)

// globalFlags contains global config values that apply for all k6 sub-commands.
type globalFlags struct {
	configFilePath string
	runType        string
	quiet          bool
	noColor        bool
	address        string
	logOutput      string
	logFormat      string
	verbose        bool
}

// globalState contains the globalFlags and accessors for most of the global
// process-external state like CLI arguments, env vars, standard input, output
// and error, etc. In practice, most of it is normally accessed through the `os`
// package from the Go stdlib.
//
// We group them here so we can prevent direct access to them from the rest of
// the k6 codebase. This gives us the ability to mock them and have robust and
// easy-to-write integration-like tests to check the k6 end-to-end behavior in
// any simulated conditions.
//
// `newGlobalState()` returns a globalState object with the real `os`
// parameters, while `newGlobalTestState()` can be used in tests to create
// simulated environments.
type globalState struct {
	ctx context.Context

	fs      afero.Fs
	getwd   func() (string, error)
	args    []string
	envVars map[string]string

	defaultFlags, flags globalFlags

	outMutex       *sync.Mutex
	stdOut, stdErr *consoleWriter
	stdIn          io.Reader

	signalNotify func(chan<- os.Signal, ...os.Signal)
	signalStop   func(chan<- os.Signal)

	// TODO: add os.Exit()?

	logger         *logrus.Logger
	fallbackLogger logrus.FieldLogger
}

// Ideally, this should be the only function in the whole codebase where we use
// global variables and functions from the os package. Anywhere else, things
// like os.Stdout, os.Stderr, os.Stdin, os.Getenv(), etc. should be removed and
// the respective properties of globalState used instead.
func newGlobalState(ctx context.Context) *globalState {
	isDumbTerm := os.Getenv("TERM") == "dumb"
	stdoutTTY := !isDumbTerm && (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
	stderrTTY := !isDumbTerm && (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd()))
	outMutex := &sync.Mutex{}
	stdOut := &consoleWriter{os.Stdout, colorable.NewColorable(os.Stdout), stdoutTTY, outMutex, nil}
	stdErr := &consoleWriter{os.Stderr, colorable.NewColorable(os.Stderr), stderrTTY, outMutex, nil}

	envVars := buildEnvMap(os.Environ())
	_, noColorsSet := envVars["NO_COLOR"] // even empty values disable colors
	logger := &logrus.Logger{
		Out: stdErr,
		Formatter: &logrus.TextFormatter{
			ForceColors:   stderrTTY,
			DisableColors: !stderrTTY || noColorsSet || envVars["K6_NO_COLOR"] != "",
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.InfoLevel,
	}

	confDir, err := os.UserConfigDir()
	if err != nil {
		logger.WithError(err).Warn("could not get config directory")
		confDir = ".config"
	}

	defaultFlags := getDefaultFlags(confDir)

	return &globalState{
		ctx:          ctx,
		fs:           afero.NewOsFs(),
		getwd:        os.Getwd,
		args:         append(make([]string, 0, len(os.Args)), os.Args...), // copy
		envVars:      envVars,
		defaultFlags: defaultFlags,
		flags:        getFlags(defaultFlags, envVars),
		outMutex:     outMutex,
		stdOut:       stdOut,
		stdErr:       stdErr,
		stdIn:        os.Stdin,
		signalNotify: signal.Notify,
		signalStop:   signal.Stop,
		logger:       logger,
		fallbackLogger: &logrus.Logger{ // we may modify the other one
			Out:       stdErr,
			Formatter: new(logrus.TextFormatter), // no fancy formatting here
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.InfoLevel,
		},
	}
}

func getDefaultFlags(homeFolder string) globalFlags {
	return globalFlags{
		address:        "localhost:6565",
		configFilePath: filepath.Join(homeFolder, "loadimpact", "k6", defaultConfigFileName),
		logOutput:      "stderr",
	}
}

func getFlags(defaultFlags globalFlags, env map[string]string) globalFlags {
	result := defaultFlags

	// TODO: add env vars for the rest of the values (after adjusting
	// rootCmdPersistentFlagSet(), of course)

	if val, ok := env["K6_CONFIG"]; ok {
		result.configFilePath = val
	}
	if val, ok := env["K6_TYPE"]; ok {
		result.runType = val
	}
	if val, ok := env["K6_LOG_OUTPUT"]; ok {
		result.logOutput = val
	}
	if val, ok := env["K6_LOG_FORMAT"]; ok {
		result.logFormat = val
	}
	if env["K6_NO_COLOR"] != "" {
		result.noColor = true
	}
	// Support https://no-color.org/, even an empty value should disable the
	// color output from k6.
	if _, ok := env["NO_COLOR"]; ok {
		result.noColor = true
	}
	return result
}

func parseEnvKeyValue(kv string) (string, string) {
	if idx := strings.IndexRune(kv, '='); idx != -1 {
		return kv[:idx], kv[idx+1:]
	}
	return kv, ""
}

func buildEnvMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v := parseEnvKeyValue(kv)
		env[k] = v
	}
	return env
}

// This is to keep all fields needed for the main/root k6 command
type rootCommand struct {
	globalState *globalState

	cmd            *cobra.Command
	loggerStopped  <-chan struct{}
	loggerIsRemote bool
}

func newRootCommand(gs *globalState) *rootCommand {
	c := &rootCommand{
		globalState: gs,
	}
	// the base command when called without any subcommands.
	rootCmd := &cobra.Command{
		Use:               "k6",
		Short:             "a next-generation load generator",
		Long:              "\n" + getBanner(c.globalState.flags.noColor || !c.globalState.stdOut.isTTY),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: c.persistentPreRunE,
	}

	rootCmd.PersistentFlags().AddFlagSet(rootCmdPersistentFlagSet(gs))
	rootCmd.SetArgs(gs.args[1:])
	rootCmd.SetOut(gs.stdOut)
	rootCmd.SetErr(gs.stdErr) // TODO: use gs.logger.WriterLevel(logrus.ErrorLevel)?
	rootCmd.SetIn(gs.stdIn)

	loginCmd := getLoginCmd()
	loginCmd.AddCommand(
		getLoginCloudCommand(gs),
		getLoginInfluxDBCommand(gs),
	)
	rootCmd.AddCommand(
		getArchiveCmd(gs), getCloudCmd(gs), getConvertCmd(gs), getInspectCmd(gs),
		loginCmd, getPauseCmd(gs), getResumeCmd(gs), getScaleCmd(gs), getRunCmd(gs),
		getStatsCmd(gs), getStatusCmd(gs), getVersionCmd(gs),
	)

	c.cmd = rootCmd
	return c
}

func (c *rootCommand) persistentPreRunE(cmd *cobra.Command, args []string) error {
	var err error

	c.loggerStopped, err = c.setupLoggers()
	if err != nil {
		return err
	}
	select {
	case <-c.loggerStopped:
	default:
		c.loggerIsRemote = true
	}

	stdlog.SetOutput(c.globalState.logger.Writer())
	c.globalState.logger.Debugf("k6 version: v%s", consts.FullVersion())
	return nil
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	globalState := newGlobalState(ctx)

	rootCmd := newRootCommand(globalState)

	if err := rootCmd.cmd.Execute(); err != nil {
		exitCode := -1
		var ecerr errext.HasExitCode
		if errors.As(err, &ecerr) {
			exitCode = int(ecerr.ExitCode())
		}

		errText := err.Error()
		var xerr errext.Exception
		if errors.As(err, &xerr) {
			errText = xerr.StackTrace()
		}

		fields := logrus.Fields{}
		var herr errext.HasHint
		if errors.As(err, &herr) {
			fields["hint"] = herr.Hint()
		}

		globalState.logger.WithFields(fields).Error(errText)
		if rootCmd.loggerIsRemote {
			globalState.fallbackLogger.WithFields(fields).Error(errText)
			cancel()
			rootCmd.waitRemoteLogger()
		}

		os.Exit(exitCode) //nolint:gocritic
	}

	cancel()
	rootCmd.waitRemoteLogger()
}

func (c *rootCommand) waitRemoteLogger() {
	if c.loggerIsRemote {
		select {
		case <-c.loggerStopped:
		case <-time.After(waitRemoteLoggerTimeout):
			c.globalState.fallbackLogger.Errorf("Remote logger didn't stop in %s", waitRemoteLoggerTimeout)
		}
	}
}

func rootCmdPersistentFlagSet(gs *globalState) *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	// TODO: refactor this config, the default value management with pflag is
	// simply terrible... :/
	//
	// We need to use `gs.flags.<value>` both as the destination and as
	// the value here, since the config values could have already been set by
	// their respective environment variables. However, we then also have to
	// explicitly set the DefValue to the respective default value from
	// `gs.defaultFlags.<value>`, so that the `k6 --help` message is
	// not messed up...

	flags.StringVar(&gs.flags.logOutput, "log-output", gs.flags.logOutput,
		"change the output for k6 logs, possible values are stderr,stdout,none,loki[=host:port],file[=./path.fileformat]")
	flags.Lookup("log-output").DefValue = gs.defaultFlags.logOutput

	flags.StringVar(&gs.flags.logFormat, "logformat", gs.flags.logFormat, "log output format")
	oldLogFormat := flags.Lookup("logformat")
	oldLogFormat.Hidden = true
	oldLogFormat.Deprecated = "log-format"
	oldLogFormat.DefValue = gs.defaultFlags.logFormat
	flags.StringVar(&gs.flags.logFormat, "log-format", gs.flags.logFormat, "log output format")
	flags.Lookup("log-format").DefValue = gs.defaultFlags.logFormat

	flags.StringVarP(&gs.flags.configFilePath, "config", "c", gs.flags.configFilePath, "JSON config file")
	// And we also need to explicitly set the default value for the usage message here, so things
	// like `K6_CONFIG="blah" k6 run -h` don't produce a weird usage message
	flags.Lookup("config").DefValue = gs.defaultFlags.configFilePath
	must(cobra.MarkFlagFilename(flags, "config"))

	flags.BoolVar(&gs.flags.noColor, "no-color", gs.flags.noColor, "disable colored output")
	flags.Lookup("no-color").DefValue = strconv.FormatBool(gs.defaultFlags.noColor)

	// TODO: support configuring these through environment variables as well?
	// either with croconf or through the hack above...
	flags.BoolVarP(&gs.flags.verbose, "verbose", "v", gs.defaultFlags.verbose, "enable verbose logging")
	flags.BoolVarP(&gs.flags.quiet, "quiet", "q", gs.defaultFlags.quiet, "disable progress updates")
	flags.StringVarP(&gs.flags.address, "address", "a", gs.defaultFlags.address, "address for the REST API server")

	return flags
}

// RawFormatter it does nothing with the message just prints it
type RawFormatter struct{}

// Format renders a single log entry
func (f RawFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}

// The returned channel will be closed when the logger has finished flushing and pushing logs after
// the provided context is closed. It is closed if the logger isn't buffering and sending messages
// Asynchronously
func (c *rootCommand) setupLoggers() (<-chan struct{}, error) {
	ch := make(chan struct{})
	close(ch)

	if c.globalState.flags.verbose {
		c.globalState.logger.SetLevel(logrus.DebugLevel)
	}

	loggerForceColors := false // disable color by default
	switch line := c.globalState.flags.logOutput; {
	case line == "stderr":
		loggerForceColors = !c.globalState.flags.noColor && c.globalState.stdErr.isTTY
		c.globalState.logger.SetOutput(c.globalState.stdErr)
	case line == "stdout":
		loggerForceColors = !c.globalState.flags.noColor && c.globalState.stdOut.isTTY
		c.globalState.logger.SetOutput(c.globalState.stdOut)
	case line == "none":
		c.globalState.logger.SetOutput(ioutil.Discard)

	case strings.HasPrefix(line, "loki"):
		ch = make(chan struct{}) // TODO: refactor, get it from the constructor
		hook, err := log.LokiFromConfigLine(c.globalState.ctx, c.globalState.fallbackLogger, line, ch)
		if err != nil {
			return nil, err
		}
		c.globalState.logger.AddHook(hook)
		c.globalState.logger.SetOutput(ioutil.Discard) // don't output to anywhere else
		c.globalState.flags.logFormat = "raw"

	case strings.HasPrefix(line, "file"):
		ch = make(chan struct{}) // TODO: refactor, get it from the constructor
		hook, err := log.FileHookFromConfigLine(c.globalState.ctx, c.globalState.fs, c.globalState.fallbackLogger, line, ch)
		if err != nil {
			return nil, err
		}

		c.globalState.logger.AddHook(hook)
		c.globalState.logger.SetOutput(ioutil.Discard)

	default:
		return nil, fmt.Errorf("unsupported log output '%s'", line)
	}

	switch c.globalState.flags.logFormat {
	case "raw":
		c.globalState.logger.SetFormatter(&RawFormatter{})
		c.globalState.logger.Debug("Logger format: RAW")
	case "json":
		c.globalState.logger.SetFormatter(&logrus.JSONFormatter{})
		c.globalState.logger.Debug("Logger format: JSON")
	default:
		c.globalState.logger.SetFormatter(&logrus.TextFormatter{
			ForceColors: loggerForceColors, DisableColors: c.globalState.flags.noColor,
		})
		c.globalState.logger.Debug("Logger format: TEXT")
	}
	return ch, nil
}
