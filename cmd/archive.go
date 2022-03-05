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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/metrics"
)

func getArchiveCmd(globalState *globalState) *cobra.Command { // nolint: funlen
	archiveOut := "archive.tar"
	// archiveCmd represents the archive command
	archiveCmd := &cobra.Command{
		Use:   "archive",
		Short: "Create an archive",
		Long: `Create an archive.

An archive is a fully self-contained test run, and can be executed identically elsewhere.`,
		Example: `
  # Archive a test run.
  k6 archive -u 10 -d 10s -O myarchive.tar script.js

  # Run the resulting archive.
  k6 run myarchive.tar`[1:],
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, filesystems, err := readSource(globalState, args[0])
			if err != nil {
				return err
			}

			runtimeOptions, err := getRuntimeOptions(cmd.Flags(), globalState.envVars)
			if err != nil {
				return err
			}

			registry := metrics.NewRegistry()
			builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
			r, err := newRunner(
				globalState.logger, src, globalState.flags.runType,
				filesystems, runtimeOptions, builtinMetrics, registry,
			)
			if err != nil {
				return err
			}

			cliOpts, err := getOptions(cmd.Flags())
			if err != nil {
				return err
			}
			conf, err := getConsolidatedConfig(globalState, Config{Options: cliOpts}, r.GetOptions())
			if err != nil {
				return err
			}

			// Parse the thresholds, only if the --no-threshold flag is not set.
			// If parsing the threshold expressions failed, consider it as an
			// invalid configuration error.
			if !runtimeOptions.NoThresholds.Bool {
				for _, thresholds := range conf.Options.Thresholds {
					err = thresholds.Parse()
					if err != nil {
						return errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
					}
				}
			}

			_, err = deriveAndValidateConfig(conf, r.IsExecutable, globalState.logger)
			if err != nil {
				return err
			}

			err = r.SetOptions(conf.Options)
			if err != nil {
				return err
			}

			// Archive.
			arc := r.MakeArchive()
			f, err := globalState.fs.Create(archiveOut)
			if err != nil {
				return err
			}

			err = arc.Write(f)
			if cerr := f.Close(); err == nil && cerr != nil {
				err = cerr
			}
			return err
		},
	}

	archiveCmd.Flags().SortFlags = false
	archiveCmd.Flags().AddFlagSet(archiveCmdFlagSet(&archiveOut))

	return archiveCmd
}

func archiveCmdFlagSet(archiveOut *string) *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	flags.StringVarP(archiveOut, "archive-out", "O", *archiveOut, "archive output filename")
	return flags
}
