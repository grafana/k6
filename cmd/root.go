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
	"fmt"
	"io"
	golog "log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/shibukawa/configdir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version contains the current semantic version of k6.
//nolint:gochecknoglobals
var Version = "0.24.0"

// Banner contains the ASCII-art banner with the k6 logo and stylized website URL
//TODO: make these into methods, only the version needs to be a variable
//nolint:gochecknoglobals
var Banner = strings.Join([]string{
	`          /\      |‾‾|  /‾‾/  /‾/   `,
	`     /\  /  \     |  |_/  /  / /    `,
	`    /  \/    \    |      |  /  ‾‾\  `,
	`   /          \   |  |‾\  \ | (_) | `,
	`  / __________ \  |__|  \__\ \___/ .io`,
}, "\n")
var BannerColor = color.New(color.FgCyan)

var (
	outMutex  = &sync.Mutex{}
	stdoutTTY = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	stderrTTY = isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
	stdout    = consoleWriter{colorable.NewColorableStdout(), stdoutTTY, outMutex}
	stderr    = consoleWriter{colorable.NewColorableStderr(), stderrTTY, outMutex}
)

const defaultConfigFileName = "config.json"

//TODO: remove these global variables
//nolint:gochecknoglobals
var defaultConfigFilePath = defaultConfigFileName // Updated with the user's config folder in the init() function below
//nolint:gochecknoglobals
var configFilePath = os.Getenv("K6_CONFIG") // Overridden by `-c`/`--config` flag!

var (
	//TODO: have environment variables for configuring these? hopefully after we move away from global vars though...
	verbose bool
	quiet   bool
	noColor bool
	logFmt  string
	address string
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:           "k6",
	Short:         "a next-generation load generator",
	Long:          BannerColor.Sprintf("\n%s", Banner),
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLoggers(logFmt)
		if noColor {
			stdout.Writer = colorable.NewNonColorable(os.Stdout)
			stderr.Writer = colorable.NewNonColorable(os.Stderr)
		}
		golog.SetOutput(log.StandardLogger().Writer())
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Error(err.Error())
		if e, ok := err.(ExitCode); ok {
			os.Exit(e.Code)
		}
		os.Exit(-1)
	}
}

func rootCmdPersistentFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	//TODO: figure out a better way to handle the CLI flags - global variables are not very testable... :/
	flags.BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
	flags.BoolVarP(&quiet, "quiet", "q", false, "disable progress updates")
	flags.BoolVar(&noColor, "no-color", false, "disable colored output")
	flags.StringVar(&logFmt, "logformat", "", "log output format")
	flags.StringVarP(&address, "address", "a", "localhost:6565", "address for the api server")

	//TODO: Fix... This default value needed, so both CLI flags and environment variables work
	flags.StringVarP(&configFilePath, "config", "c", configFilePath, "JSON config file")
	// And we also need to explicitly set the default value for the usage message here, so things
	// like `K6_CONFIG="blah" k6 run -h` don't produce a weird usage message
	flags.Lookup("config").DefValue = defaultConfigFilePath
	must(cobra.MarkFlagFilename(flags, "config"))
	return flags
}

func init() {
	// TODO: find a better library... or better yet, simply port the few dozen lines of code for getting the
	// per-user config folder in a cross-platform way
	configDirs := configdir.New("loadimpact", "k6")
	configFolders := configDirs.QueryFolders(configdir.Global)
	if len(configFolders) > 0 {
		defaultConfigFilePath = filepath.Join(configFolders[0].Path, defaultConfigFileName)
	}

	RootCmd.PersistentFlags().AddFlagSet(rootCmdPersistentFlagSet())
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
type RawFormater struct{}

// Format renders a single log entry
func (f RawFormater) Format(entry *log.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}

func setupLoggers(logFmt string) {
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
	log.SetOutput(stderr)

	switch logFmt {
	case "raw":
		log.SetFormatter(&RawFormater{})
		log.Debug("Logger format: RAW")
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
		log.Debug("Logger format: JSON")
	default:
		log.SetFormatter(&log.TextFormatter{ForceColors: stderrTTY, DisableColors: noColor})
		log.Debug("Logger format: TEXT")
	}

}
