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
)

func getArchiveCmd(gs *globalState) *cobra.Command {
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
			test, err := loadTest(gs, cmd, args, getPartialConfig)
			if err != nil {
				return err
			}

			// It's important to NOT set the derived options back to the runner
			// here, only the consolidated ones. Otherwise, if the script used
			// an execution shortcut option (e.g. `iterations` or `duration`),
			// we will have multiple conflicting execution options since the
			// derivation will set `scenarios` as well.
			err = test.initRunner.SetOptions(test.consolidatedConfig.Options)
			if err != nil {
				return err
			}

			// Archive.
			arc := test.initRunner.MakeArchive()
			f, err := gs.fs.Create(archiveOut)
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
