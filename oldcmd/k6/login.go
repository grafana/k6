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
	"os"

	"github.com/loadimpact/k6/lib"
	"gopkg.in/urfave/cli.v1"
)

var commandLogin = cli.Command{
	Name:  "login",
	Usage: "Logs into a remote service.",
	Subcommands: cli.Commands{
		cli.Command{
			Name:   "influxdb",
			Usage:  "Logs into an influxdb server.",
			Action: actionLogin(CollectorInfluxDB),
		},
	},
}

func actionLogin(t string) func(cc *cli.Context) error {
	return func(cc *cli.Context) error {
		conf, err := LoadConfig()
		if err != nil {
			return err
		}

		c := collectorOfType(t).(lib.AuthenticatedCollector)

		cconf, err := c.Login(conf.Collectors.Get(t), os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		conf.Collectors[t] = cconf

		return conf.Store()
	}
}
