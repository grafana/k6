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

package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/log"
)

var BannerColor = color.New(color.FgCyan)

//TODO: remove these global variables
//nolint:gochecknoglobals
var (
	outMutex  = &sync.Mutex{}
	stdoutTTY = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	stderrTTY = isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
	stdout    = &consoleWriter{colorable.NewColorableStdout(), stdoutTTY, outMutex, nil}
	stderr    = &consoleWriter{colorable.NewColorableStderr(), stderrTTY, outMutex, nil}
)

const (
	defaultConfigFileName   = "config.json"
	waitRemoteLoggerTimeout = time.Second * 5
)

//TODO: remove these global variables
//nolint:gochecknoglobals
var defaultConfigFilePath = defaultConfigFileName // Updated with the user's config folder in the init() function below
//nolint:gochecknoglobals
var configFilePath = os.Getenv("K6_CONFIG") // Overridden by `-c`/`--config` flag!

//nolint:gochecknoglobals
var (
	// TODO: have environment variables for configuring these? hopefully after we move away from global vars though...
	quiet   bool
	noColor bool
	address string
)

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
}

func newRootCommand(ctx context.Context, logger *logrus.Logger, fallbackLogger logrus.FieldLogger) *rootCommand {
	c := &rootCommand{
		ctx:            ctx,
		logger:         logger,
		fallbackLogger: fallbackLogger,
	}
	// the base command when called without any subcommands.
	c.cmd = &cobra.Command{
		Use:               "k6",
		Short:             "a next-generation load generator",
		Long:              BannerColor.Sprintf("\n%s", consts.Banner()),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: c.persistentPreRunE,
	}

	confDir, err := os.UserConfigDir()
	if err != nil {
		logrus.WithError(err).Warn("could not get config directory")
		confDir = ".config"
	}
	defaultConfigFilePath = filepath.Join(
		confDir,
		"loadimpact",
		"k6",
		defaultConfigFileName,
	)

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

	if noColor {
		// TODO: figure out something else... currently, with the wrappers
		// below, we're stripping any colors from the output after we've
		// added them. The problem is that, besides being very inefficient,
		// this actually also strips other special characters from the
		// intended output, like the progressbar formatting ones, which
		// would otherwise be fine (in a TTY).
		//
		// It would be much better if we avoid messing with the output and
		// instead have a parametrized instance of the color library. It
		// will return colored output if colors are enabled and simply
		// return the passed input as-is (i.e. be a noop) if colors are
		// disabled...
		stdout.Writer = colorable.NewNonColorable(os.Stdout)
		stderr.Writer = colorable.NewNonColorable(os.Stderr)
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
	loginCmd.AddCommand(getLoginCloudCommand(logger), getLoginInfluxDBCommand(logger))
	c.cmd.AddCommand(
		getArchiveCmd(logger),
		getCloudCmd(ctx, logger),
		getConvertCmd(),
		getInspectCmd(logger),
		loginCmd,
		getPauseCmd(ctx),
		getResumeCmd(ctx),
		getScaleCmd(ctx),
		getRunCmd(ctx, logger),
		getStatsCmd(ctx),
		getStatusCmd(ctx),
		getVersionCmd(),
	)

	if err := c.cmd.Execute(); err != nil {
		fields := logrus.Fields{}
		code := -1
		if e, ok := err.(ExitCode); ok {
			code = e.Code
			if e.Hint != "" {
				fields["hint"] = e.Hint
			}
		}

		logger.WithFields(fields).Error(err)
		if c.loggerIsRemote {
			fallbackLogger.WithFields(fields).Error(err)
			cancel()
			c.waitRemoteLogger()
		}

		os.Exit(code)
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
	flags.BoolVarP(&c.verbose, "verbose", "v", false, "enable debug logging")
	flags.BoolVarP(&quiet, "quiet", "q", false, "disable progress updates")
	flags.BoolVar(&noColor, "no-color", false, "disable colored output")
	flags.StringVar(&c.logOutput, "log-output", "stderr",
		"change the output for k6 logs, possible values are stderr,stdout,none,loki[=host:port]")
	flags.StringVar(&c.logFmt, "logformat", "", "log output format") // TODO rename to log-format and warn on old usage
	flags.StringVarP(&address, "address", "a", "localhost:6565", "address for the api server")

	// TODO: Fix... This default value needed, so both CLI flags and environment variables work
	flags.StringVarP(&configFilePath, "config", "c", configFilePath, "JSON config file")
	// And we also need to explicitly set the default value for the usage message here, so things
	// like `K6_CONFIG="blah" k6 run -h` don't produce a weird usage message
	flags.Lookup("config").DefValue = defaultConfigFilePath
	must(cobra.MarkFlagFilename(flags, "config"))
	return flags
}

// fprintf panics when where's an error writing to the supplied io.Writer
func fprintf(w io.Writer, format string, a ...interface{}) (n int) {
	n, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		panic(err.Error())
	}
	return n
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
	switch c.logOutput {
	case "stderr":
		c.logger.SetOutput(stderr)
	case "stdout":
		c.logger.SetOutput(stdout)
	case "none":
		c.logger.SetOutput(ioutil.Discard)
	default:
		if !strings.HasPrefix(c.logOutput, "loki") {
			return nil, fmt.Errorf("unsupported log output `%s`", c.logOutput)
		}
		ch = make(chan struct{})
		hook, err := log.LokiFromConfigLine(c.ctx, c.fallbackLogger, c.logOutput, ch)
		if err != nil {
			return nil, err
		}
		c.logger.AddHook(hook)
		c.logger.SetOutput(ioutil.Discard) // don't output to anywhere else
		c.logFmt = "raw"
		noColor = true // disable color
	}

	switch c.logFmt {
	case "raw":
		c.logger.SetFormatter(&RawFormatter{})
		c.logger.Debug("Logger format: RAW")
	case "json":
		c.logger.SetFormatter(&logrus.JSONFormatter{})
		c.logger.Debug("Logger format: JSON")
	default:
		c.logger.SetFormatter(&logrus.TextFormatter{ForceColors: stderrTTY, DisableColors: noColor})
		c.logger.Debug("Logger format: TEXT")
	}
	return ch, nil
}
