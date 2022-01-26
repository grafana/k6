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
	"os"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/output/influxdb"
	"go.k6.io/k6/ui"
)

//nolint:funlen
func getLoginInfluxDBCommand(logger logrus.FieldLogger, globalFlags *commandFlags) *cobra.Command {
	// loginInfluxDBCommand represents the 'login influxdb' command
	loginInfluxDBCommand := &cobra.Command{
		Use:   "influxdb [uri]",
		Short: "Authenticate with InfluxDB",
		Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fs := afero.NewOsFs()
			config, configPath, err := readDiskConfig(fs, globalFlags)
			if err != nil {
				return err
			}

			conf := influxdb.NewConfig()
			jsonConf := config.Collectors["influxdb"]
			if jsonConf != nil {
				jsonConfParsed, jsonerr := influxdb.ParseJSON(jsonConf)
				if jsonerr != nil {
					return jsonerr
				}
				conf = conf.Apply(jsonConfParsed)
			}
			if len(args) > 0 {
				urlConf, err := influxdb.ParseURL(args[0]) //nolint:govet
				if err != nil {
					return err
				}
				conf = conf.Apply(urlConf)
			}

			form := ui.Form{
				Fields: []ui.Field{
					ui.StringField{
						Key:     "Addr",
						Label:   "Address",
						Default: conf.Addr.String,
					},
					ui.StringField{
						Key:     "DB",
						Label:   "Database",
						Default: conf.DB.String,
					},
					ui.StringField{
						Key:     "Username",
						Label:   "Username",
						Default: conf.Username.String,
					},
					ui.PasswordField{
						Key:   "Password",
						Label: "Password",
					},
				},
			}
			if !term.IsTerminal(int(syscall.Stdin)) { // nolint: unconvert
				logger.Warn("Stdin is not a terminal, falling back to plain text input")
			}
			vals, err := form.Run(os.Stdin, stdout)
			if err != nil {
				return err
			}

			conf.Addr = null.StringFrom(vals["Addr"])
			conf.DB = null.StringFrom(vals["DB"])
			conf.Username = null.StringFrom(vals["Username"])
			conf.Password = null.StringFrom(vals["Password"])

			client, err := influxdb.MakeClient(conf)
			if err != nil {
				return err
			}
			if _, _, err = client.Ping(10 * time.Second); err != nil {
				return err
			}

			if config.Collectors == nil {
				config.Collectors = make(map[string]json.RawMessage)
			}
			config.Collectors["influxdb"], err = json.Marshal(conf)
			if err != nil {
				return err
			}
			return writeDiskConfig(fs, configPath, config)
		},
	}
	return loginInfluxDBCommand
}
