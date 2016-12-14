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

package main

import (
	"gopkg.in/urfave/cli.v1"
)

var commandStatus = cli.Command{
	Name:      "status",
	Usage:     "Looks up the status of a running test",
	ArgsUsage: " ",
	Action:    actionStatus,
	Description: `Status will print the status of a running test to stdout in YAML format.

   Use the global --address/-a flag to specify the host to connect to; the
   default is port 6565 on the local machine.

   Endpoint: /v1/status`,
}

var commandStats = cli.Command{
	Name:      "stats",
	Usage:     "Prints stats for a running test",
	ArgsUsage: "[name]",
	Action:    actionStats,
	Description: `Stats will print metrics about a running test to stdout in YAML format.

   The result is a dictionary of metrics. If a name is specified, only that one
   metric is fetched, otherwise every metric is printed in no particular order.

   Endpoint: /v1/metrics
             /v1/metrics/:id`,
}

var commandScale = cli.Command{
	Name:      "scale",
	Usage:     "Scales a running test",
	ArgsUsage: "vus",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "max, m",
			Usage: "update the max number of VUs allowed",
		},
	},
	Action: actionScale,
	Description: `Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test. Use --max if you want to do this.

   Endpoint: /v1/status`,
}

var commandPause = cli.Command{
	Name:      "pause",
	Usage:     "Pauses a running test",
	ArgsUsage: " ",
	Action:    actionPause,
	Description: `Pause pauses a running test.

   Running VUs will finish their current iterations, then suspend themselves
   until woken by the test's resumption. A sleeping VU will consume no CPU
   cycles, but will still occupy memory.

   Endpoint: /v1/status`,
}

var commandStart = cli.Command{
	Name:      "start",
	Usage:     "Starts a paused test",
	ArgsUsage: " ",
	Action:    actionStart,
	Description: `Start starts a paused test.

   This is the opposite of the pause command, and will do nothing to an already
   running test.

   Endpoint: /v1/status`,
}

func actionStatus(cc *cli.Context) error {
	return nil
}

func actionStats(cc *cli.Context) error {
	return nil
}

func actionScale(cc *cli.Context) error {
	return nil
}

func actionPause(cc *cli.Context) error {
	return nil
}

func actionStart(cc *cli.Context) error {
	return nil
}
