/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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
	"encoding/json"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/converter/har"
	"github.com/loadimpact/k6/lib"
)

var (
	output              string
	optionsFilePath     string
	minSleep            uint
	maxSleep            uint
	enableChecks        bool
	returnOnFailedCheck bool
	correlate           bool
	threshold           uint
	nobatch             bool
	only                []string
	skip                []string
)

//nolint:funlen,gocognit
func getConvertCmd() *cobra.Command {
	convertCmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert a HAR file to a k6 script",
		Long:  "Convert a HAR (HTTP Archive) file to a k6 script",
		Example: `
  # Convert a HAR file to a k6 script.
  k6 convert -O har-session.js session.har

  # Convert a HAR file to a k6 script creating requests only for the given domain/s.
  k6 convert -O har-session.js --only yourdomain.com,additionaldomain.com session.har

  # Convert a HAR file. Batching requests together as long as idle time between requests <800ms
  k6 convert --batch-threshold 800 session.har

  # Run the k6 script.
  k6 run har-session.js`[1:],
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse the HAR file
			filePath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			r, err := defaultFs.Open(filePath)
			if err != nil {
				return err
			}
			h, err := har.Decode(r)
			if err != nil {
				return err
			}
			if err = r.Close(); err != nil {
				return err
			}

			// recordings include redirections as separate requests, and we dont want to trigger them twice
			options := lib.Options{MaxRedirects: null.IntFrom(0)}

			if optionsFilePath != "" {
				optionsFileContents, err := ioutil.ReadFile(optionsFilePath) //nolint:gosec,govet
				if err != nil {
					return err
				}
				var injectedOptions lib.Options
				if err := json.Unmarshal(optionsFileContents, &injectedOptions); err != nil {
					return err
				}
				options = options.Apply(injectedOptions)
			}

			// TODO: refactor...
			script, err := har.Convert(h, options, minSleep, maxSleep, enableChecks,
				returnOnFailedCheck, threshold, nobatch, correlate, only, skip)
			if err != nil {
				return err
			}

			// Write script content to stdout or file
			if output == "" || output == "-" { //nolint:nestif
				if _, err := io.WriteString(defaultWriter, script); err != nil {
					return err
				}
			} else {
				f, err := defaultFs.Create(output)
				if err != nil {
					return err
				}
				if _, err := f.WriteString(script); err != nil {
					return err
				}
				if err := f.Sync(); err != nil {
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			}
			return nil
		},
	}

	convertCmd.Flags().SortFlags = false
	convertCmd.Flags().StringVarP(&output, "output", "O", output, "k6 script output filename (stdout by default)")
	convertCmd.Flags().StringVarP(&optionsFilePath, "options", "", output, "path to a JSON file with options that would be injected in the output script")
	convertCmd.Flags().StringSliceVarP(&only, "only", "", []string{}, "include only requests from the given domains")
	convertCmd.Flags().StringSliceVarP(&skip, "skip", "", []string{}, "skip requests from the given domains")
	convertCmd.Flags().UintVarP(&threshold, "batch-threshold", "", 500, "batch request idle time threshold (see example)")
	convertCmd.Flags().BoolVarP(&nobatch, "no-batch", "", false, "don't generate batch calls")
	convertCmd.Flags().BoolVarP(&enableChecks, "enable-status-code-checks", "", false, "add a status code check for each HTTP response")
	convertCmd.Flags().BoolVarP(&returnOnFailedCheck, "return-on-failed-check", "", false, "return from iteration if we get an unexpected response status code")
	convertCmd.Flags().BoolVarP(&correlate, "correlate", "", false, "detect values in responses being used in subsequent requests and try adapt the script accordingly (only redirects and JSON values for now)")
	convertCmd.Flags().UintVarP(&minSleep, "min-sleep", "", 20, "the minimum amount of seconds to sleep after each iteration")
	convertCmd.Flags().UintVarP(&maxSleep, "max-sleep", "", 40, "the maximum amount of seconds to sleep after each iteration")
	return convertCmd
}
