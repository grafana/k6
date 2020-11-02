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

	"github.com/spf13/cobra"

	"github.com/loadimpact/k6/lib/consts"
)

func getVersionCmd() *cobra.Command {
	// versionCmd represents the version command.
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("k6 v" + consts.FullVersion())
		},
	}
	return versionCmd
}
