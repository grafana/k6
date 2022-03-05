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
	"io/ioutil"
	stdlog "log"
	"os"
	"path/filepath"
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

// TODO better name - there are other command flags these are just ... non lib.Options ones :shrug:
type commandFlags struct {
	defaultConfigFilePath string
	configFilePath        string
	exitOnRunning         bool
	showCloudLogs         bool
	runType               string
	archiveOut            string
	quiet                 bool
	noColor               bool
	address               string
	outMutex              *sync.Mutex
	stdoutTTY, stderrTTY  bool
	stdout, stderr        *consoleWriter
}

func newCommandFlags() *commandFlags {
	confDir, err := os.UserConfigDir()
	if err != nil {
		logrus.WithError(err).Warn("could not get config directory")
		confDir = ".config"
	}
	defaultConfigFilePath := filepath.Join(
		confDir,
		"loadimpact",
		"k6",
		defaultConfigFileName,
	)

	isDumbTerm := os.Getenv("TERM") == "dumb"
	stdoutTTY := !isDumbTerm && (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()))
	stderrTTY := !isDumbTerm && (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd()))
	outMutex := &sync.Mutex{}
	return &commandFlags{
		defaultConfigFilePath: defaultConfigFilePath,  // Updated with the user's config folder in the init() function below
		configFilePath:        os.Getenv("K6_CONFIG"), // Overridden by `-c`/`--config` flag!
		exitOnRunning:         os.Getenv("K6_EXIT_ON_RUNNING") != "",
		showCloudLogs:         true,
		runType:               os.Getenv("K6_TYPE"),
		archiveOut:            "archive.tar",
		outMutex:              outMutex,
		stdoutTTY:             stdoutTTY,
		stderrTTY:             stderrTTY,
		stdout:                &consoleWriter{colorable.NewColorableStdout(), stdoutTTY, outMutex, nil},
		stderr:                &consoleWriter{colorable.NewColorableStderr(), stderrTTY, outMutex, nil},
	}
}

// This is to keep all fields needed for the main/root k6 command
type rootCommand struct {
	ctx            context.Context
	logger         *logrus.Logger
	fallbackLogger logrus.FieldLogger
	cmd            *cobra.Command
	loggerStopped  <-chan struct{}
	logOutput      string
	logFmt         string
	loggerIsRemote bool
	verbose        bool
	h3             bool
	commandFlags   *commandFlags
}

func newRootCommand(ctx context.Context, logger *logrus.Logger, fallbackLogger logrus.FieldLogger) *rootCommand {
	c := &rootCommand{
		ctx:            ctx,
		logger:         logger,
		fallbackLogger: fallbackLogger,
		commandFlags:   newCommandFlags(),
	}
	// the base command when called without any subcommands.
	c.cmd = &cobra.Command{
		Use:               "k6",
		Short:             "a next-generation load generator",
		Long:              "\n" + getBanner(c.commandFlags.noColor || !c.commandFlags.stdoutTTY),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: c.persistentPreRunE,
	}

	c.cmd.PersistentFlags().AddFlagSet(c.rootCmdPersistentFlagSet())
	return c
}

func (c *rootCommand) persistentPreRunE(cmd *cobra.Command, args []string) error {
	var err error
	if !cmd.Flags().Changed("log-output") {
		if envLogOutput, ok := os.LookupEnv("K6_LOG_OUTPUT"); ok {
			c.logOutput = envLogOutput
		}
	}
	c.loggerStopped, err = c.setupLoggers()
	if err != nil {
		return err
	}
	select {
	case <-c.loggerStopped:
	default:
		c.loggerIsRemote = true
	}

	stdlog.SetOutput(c.logger.Writer())
	c.logger.Debugf("k6 version: v%s", consts.FullVersion())
	return nil
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
	}

	var fallbackLogger logrus.FieldLogger = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
	}

	c := newRootCommand(ctx, logger, fallbackLogger)

	loginCmd := getLoginCmd()
	loginCmd.AddCommand(
		getLoginCloudCommand(logger, c.commandFlags),
		getLoginInfluxDBCommand(logger, c.commandFlags),
	)
	c.cmd.AddCommand(
		getArchiveCmd(logger, c.commandFlags),
		getCloudCmd(ctx, logger, c.commandFlags),
		getConvertCmd(afero.NewOsFs(), c.commandFlags.stdout),
		getInspectCmd(logger, c.commandFlags),
		loginCmd,
		getPauseCmd(ctx, c.commandFlags),
		getResumeCmd(ctx, c.commandFlags),
		getScaleCmd(ctx, c.commandFlags),
		getRunCmd(ctx, logger, c.commandFlags),
		getStatsCmd(ctx, c.commandFlags),
		getStatusCmd(ctx, c.commandFlags),
		getVersionCmd(),
	)

	if err := c.cmd.Execute(); err != nil {
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

		logger.WithFields(fields).Error(errText)
		if c.loggerIsRemote {
			fallbackLogger.WithFields(fields).Error(errText)
			cancel()
			c.waitRemoteLogger()
		}

		os.Exit(exitCode) //nolint:gocritic
	}

	cancel()
	c.waitRemoteLogger()
}

func (c *rootCommand) waitRemoteLogger() {
	if c.loggerIsRemote {
		select {
		case <-c.loggerStopped:
		case <-time.After(waitRemoteLoggerTimeout):
			c.fallbackLogger.Error("Remote logger didn't stop in %s", waitRemoteLoggerTimeout)
		}
	}
}

func (c *rootCommand) rootCmdPersistentFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	// TODO: figure out a better way to handle the CLI flags - global variables are not very testable... :/
	flags.BoolVarP(&c.verbose, "verbose", "v", false, "enable verbose logging")
	flags.BoolVarP(&c.h3, "http3", "3", false, "enable http3")
	flags.BoolVarP(&c.commandFlags.quiet, "quiet", "q", false, "disable progress updates")
	flags.BoolVar(&c.commandFlags.noColor, "no-color", false, "disable colored output")
	flags.StringVar(&c.logOutput, "log-output", "stderr",
		"change the output for k6 logs, possible values are stderr,stdout,none,loki[=host:port],file[=./path.fileformat]")
	flags.StringVar(&c.logFmt, "logformat", "", "log output format") // TODO rename to log-format and warn on old usage
	flags.StringVarP(&c.commandFlags.address, "address", "a", "localhost:6565", "address for the api server")

	// TODO: Fix... This default value needed, so both CLI flags and environment variables work
	flags.StringVarP(&c.commandFlags.configFilePath, "config", "c", c.commandFlags.configFilePath, "JSON config file")
	// And we also need to explicitly set the default value for the usage message here, so things
	// like `K6_CONFIG="blah" k6 run -h` don't produce a weird usage message
	flags.Lookup("config").DefValue = c.commandFlags.defaultConfigFilePath
	must(cobra.MarkFlagFilename(flags, "config"))
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

	if c.verbose {
		c.logger.SetLevel(logrus.DebugLevel)
	}

	loggerForceColors := false // disable color by default
	switch line := c.logOutput; {
	case line == "stderr":
		loggerForceColors = !c.commandFlags.noColor && c.commandFlags.stderrTTY
		c.logger.SetOutput(c.commandFlags.stderr)
	case line == "stdout":
		loggerForceColors = !c.commandFlags.noColor && c.commandFlags.stdoutTTY
		c.logger.SetOutput(c.commandFlags.stdout)
	case line == "none":
		c.logger.SetOutput(ioutil.Discard)

	case strings.HasPrefix(line, "loki"):
		ch = make(chan struct{})
		hook, err := log.LokiFromConfigLine(c.ctx, c.fallbackLogger, line, ch)
		if err != nil {
			return nil, err
		}
		c.logger.AddHook(hook)
		c.logger.SetOutput(ioutil.Discard) // don't output to anywhere else
		c.logFmt = "raw"

	case strings.HasPrefix(line, "file"):
		hook, err := log.FileHookFromConfigLine(c.ctx, c.fallbackLogger, line)
		if err != nil {
			return nil, err
		}

		c.logger.AddHook(hook)
		c.logger.SetOutput(ioutil.Discard)

	default:
		return nil, fmt.Errorf("unsupported log output `%s`", line)
	}

	switch c.logFmt {
	case "raw":
		c.logger.SetFormatter(&RawFormatter{})
		c.logger.Debug("Logger format: RAW")
	case "json":
		c.logger.SetFormatter(&logrus.JSONFormatter{})
		c.logger.Debug("Logger format: JSON")
	default:
		c.logger.SetFormatter(&logrus.TextFormatter{ForceColors: loggerForceColors, DisableColors: c.commandFlags.noColor})
		c.logger.Debug("Logger format: TEXT")
	}
	return ch, nil
}
