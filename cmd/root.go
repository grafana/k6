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
	"os"
	"path/filepath"
	"sync"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/shibukawa/configdir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var Version = "0.21.0-dev"
var Banner = `
          /\      |‾‾|  /‾‾/  /‾/   
     /\  /  \     |  |_/  /  / /   
    /  \/    \    |      |  /  ‾‾\  
   /          \   |  |‾\  \ | (_) | 
  / __________ \  |__|  \__\ \___/ .io`

var BannerColor = color.New(color.FgCyan)

var (
	outMutex  = &sync.Mutex{}
	stdoutTTY = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	stderrTTY = isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
	stdout    = consoleWriter{colorable.NewColorableStdout(), stdoutTTY, outMutex}
	stderr    = consoleWriter{colorable.NewColorableStderr(), stderrTTY, outMutex}
)

var (
	cfgFile string

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
	Long:          BannerColor.Sprint(Banner),
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLoggers(logFmt)
		if noColor {
			stdout.Writer = colorable.NewNonColorable(os.Stdout)
			stdout.Writer = colorable.NewNonColorable(os.Stderr)
		}
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

func init() {
	defaultConfigPathMsg := ""
	configFolders := configDirs.QueryFolders(configdir.Global)
	if len(configFolders) > 0 {
		defaultConfigPathMsg = fmt.Sprintf(" (default %s)", filepath.Join(configFolders[0].Path, configFilename))
	}

	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "disable progress updates")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	RootCmd.PersistentFlags().StringVar(&logFmt, "logformat", "", "log output format")
	RootCmd.PersistentFlags().StringVarP(&address, "address", "a", "localhost:6565", "address for the api server")
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file"+defaultConfigPathMsg)
	must(cobra.MarkFlagFilename(RootCmd.PersistentFlags(), "config"))
}

func setupLoggers(logFmt string) {
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
	log.SetOutput(stderr)

	switch logFmt {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
		log.Debug("Logger format: JSON")
	default:
		log.SetFormatter(&log.TextFormatter{ForceColors: stderrTTY})
		log.Debug("Logger format: TEXT")
	}

}
