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
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// inspectCmd represents the resume command
var inspectCmd = &cobra.Command{
	Use:   "inspect [file]",
	Short: "Inspect a script or archive",
	Long:  `Inspect a script or archive.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		fs := afero.NewOsFs()
		src, err := readSource(args[0], pwd, fs, os.Stdin)
		if err != nil {
			return err
		}

		typ := runType
		if typ == "" {
			typ = detectType(src.Data)
		}

		var opts lib.Options
		switch typ {
		case typeArchive:
			arc, err := lib.ReadArchive(bytes.NewBuffer(src.Data))
			if err != nil {
				return err
			}
			opts = arc.Options
		case typeJS:
			b, err := js.NewBundle(src, fs)
			if err != nil {
				return err
			}
			opts = b.Options
		}

		data, err := json.MarshalIndent(opts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().StringVarP(&runType, "type", "t", runType, "override file `type`, \"js\" or \"archive\"")
}
