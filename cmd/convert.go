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
        "path/filepath"

        "github.com/pkg/errors"
        "github.com/spf13/cobra"

        "bufio"
        "bytes"
        "fmt"
        "os"

        "github.com/loadimpact/k6/converter/har"
)

var (
        output           string
        enableChecks     bool
        threshold        uint
        only             []string
        skip             []string
        maxRequestsBatch uint
)

var convertCmd = &cobra.Command{
        Use:   "convert",
        Short: "Converts a HAR file to a k6 js script",
        Long: `Converts a HAR (HTTP Archive) file to a k6 script.

  By default the HTTP requests are grouped by their start time in intervals
  of 500ms. You can modify this value with the flag --batch-inclusion-threshold.

  --only and --skip flags allow you to filter HAR HTTP requests and generate a
  k6 script with the desired requests: --only domain.com`,
        RunE: func(cmd *cobra.Command, args []string) error {
                if len(args) < 1 {
                        return errors.New("Must specify a HAR file as parameter!")
                }

                // handles absolute/relative paths
                abs, err := filepath.Abs(args[0])
                if err != nil {
                        return errors.New(err.Error())
                }

                // parse HAR file
                h, err := har.Read(abs)
                if err != nil {
                        return errors.New(err.Error())
                }

                var b bytes.Buffer
                w := bufio.NewWriter(&b)

                err = har.WriteK6Script(
                        w,
                        h,
                        enableChecks,
                        threshold,
                        only,
                        skip,
                        maxRequestsBatch,
                )
                if err != nil {
                        return errors.New(err.Error())
                }

                // output filename
                if output == "" {
                        basename := filepath.Base(abs)
                        output = fmt.Sprintf("%v.js", basename[0:len(basename)-len(filepath.Ext(basename))])
                }

                f, err := os.Create(output)
                if err != nil {
                        return errors.New("Can't create output filename")
                }
                if _, err = f.Write(b.Bytes()); err != nil {
                        return errors.New(err.Error())
                }
                if err = f.Sync(); err != nil {
                        return errors.New(err.Error())
                }
                if err = f.Close(); err != nil {
                        return errors.New(err.Error())
                }

                return nil
        },
}

func init() {
        RootCmd.AddCommand(convertCmd)
        convertCmd.Flags().StringVarP(&output, "output", "o", "", "filename for the output k6 script file")
        convertCmd.Flags().BoolVarP(&enableChecks, "enable-status-code-checks", "", false, "add a status code check in every request")
        convertCmd.Flags().UintVarP(&threshold, "batch-inclusion-threshold", "", 500, "batch inclusion threshold")
        convertCmd.Flags().StringSliceVarP(&only, "only", "", []string{}, "include only requests from the given domains")
        convertCmd.Flags().StringSliceVarP(&skip, "skip", "", []string{}, "skip requests from the given domains")
        convertCmd.Flags().UintVarP(&maxRequestsBatch, "max-requests-batch", "", 5, "max number of requests in a batch statement")
}
