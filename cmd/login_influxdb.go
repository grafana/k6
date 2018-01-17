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
	"os"
	"time"

	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/ui"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// loginInfluxDBCommand represents the 'login influxdb' command
var loginInfluxDBCommand = &cobra.Command{
	Use:   "influxdb [uri]",
	Short: "Authenticate with InfluxDB",
	Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fs := afero.NewOsFs()
		config, cdir, err := readDiskConfig(fs)
		if err != nil {
			return err
		}

		conf := config.Collectors.InfluxDB
		if len(args) > 0 {
			if err := conf.UnmarshalText([]byte(args[0])); err != nil {
				return err
			}
		}
		if conf.Addr == "" {
			conf.Addr = "http://localhost:8086"
		}
		if conf.DB == "" {
			conf.DB = "k6"
		}

		form := ui.Form{
			Fields: []ui.Field{
				ui.StringField{
					Key:     "Addr",
					Label:   "Address",
					Default: conf.Addr,
				},
				ui.StringField{
					Key:     "DB",
					Label:   "Database",
					Default: conf.DB,
				},
				ui.StringField{
					Key:     "Username",
					Label:   "Username",
					Default: conf.Username,
				},
				ui.StringField{
					Key:     "Password",
					Label:   "Password",
					Default: conf.Password,
				},
			},
		}
		vals, err := form.Run(os.Stdin, stdout)
		if err != nil {
			return err
		}
		if err := mapstructure.Decode(vals, &conf); err != nil {
			return err
		}

		coll, err := influxdb.New(conf)
		if err != nil {
			return err
		}
		if _, _, err := coll.Client.Ping(10 * time.Second); err != nil {
			return err
		}

		config.Collectors.InfluxDB = conf
		return writeDiskConfig(fs, cdir, config)
	},
}

func init() {
	loginCmd.AddCommand(loginInfluxDBCommand)
}
