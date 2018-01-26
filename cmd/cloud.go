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
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/ui"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Run a test on the cloud",
	Long: `Run a test on the cloud.

This will execute the test on the Load Impact cloud service. Use "k6 login cloud" to authenticate.`,
	Example: `
        k6 cloud script.js`[1:],
	Args: exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = BannerColor.Fprint(stdout, Banner+"\n\n")
		fmt.Fprint(stdout, "  Uploading script to the cloud..")

		// Runner
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}

		filename := args[0]
		src, err := readSource(filename, pwd, afero.NewOsFs(), os.Stdin)
		if err != nil {
			return err
		}

		r, err := newRunner(src, runType, afero.NewOsFs())
		if err != nil {
			return err
		}

		// Options
		fs := afero.NewOsFs()
		fileConf, _, err := readDiskConfig(fs)
		if err != nil {
			return err
		}
		options, err := getOptions(cmd.Flags())
		if err != nil {
			return err
		}
		cliConf := Config{Options: options}

		envConf, err := readEnvConfig()
		if err != nil {
			return err
		}
		conf := cliConf.Apply(fileConf).Apply(Config{Options: r.GetOptions()}).Apply(envConf).Apply(cliConf)
		r.SetOptions(conf.Options)

		// Cloud config
		cloudConfig := conf.Collectors.Cloud
		if err := envconfig.Process("k6", &cloudConfig); err != nil {
			return err
		}
		if cloudConfig.Token == "" {
			return errors.New("Not logged in, please use `k6 login cloud`.")
		}

		// Create a ticker to add a dot to the console every 0.5s
		ticker := time.NewTicker(time.Millisecond * 500)
		go func() {
			for range ticker.C {
				fmt.Fprint(stdout, ".")
			}
		}()

		// Start cloud test run
		client := cloud.NewClient(cloudConfig.Token, cloudConfig.Host, Version)

		arc := r.MakeArchive()
		if err := client.ValidateOptions(arc.Options); err != nil {
			return err
		}

		if val, ok := arc.Options.External["loadimpact"]; ok {
			if err := mapstructure.Decode(val, &cloudConfig); err != nil {
				return err
			}
		}
		name := cloudConfig.Name
		if name == "" {
			name = filepath.Base(filename)
		}

		refID, err := client.StartCloudTestRun(name, cloudConfig.ProjectID, arc)
		if err != nil {
			return err
		}
		ticker.Stop()

		testURL := cloud.URLForResults(refID, cloudConfig)
		fmt.Fprint(stdout, "\n\n")
		fmt.Fprintf(stdout, "     execution: %s\n", ui.ValueColor.Sprint("cloud"))
		fmt.Fprintf(stdout, "     script: %s\n", ui.ValueColor.Sprint(filename))
		fmt.Fprintf(stdout, "     output: %s\n", ui.ValueColor.Sprint(testURL))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(cloudCmd)
	cloudCmd.Flags().AddFlagSet(optionFlagSet())
}
