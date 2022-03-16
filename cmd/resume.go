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
	"gopkg.in/guregu/null.v3"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/api/v1/client"
)

func getCmdResume(globalState *globalState) *cobra.Command {
	// resumeCmd represents the resume command
	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a paused test",
		Long: `Resume a paused test.

  Use the global --address flag to specify the URL to the API server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(globalState.flags.address)
			if err != nil {
				return err
			}
			status, err := c.SetStatus(globalState.ctx, v1.Status{
				Paused: null.BoolFrom(false),
			})
			if err != nil {
				return err
			}

			return yamlPrint(globalState.stdOut, status)
		},
	}
	return resumeCmd
}
