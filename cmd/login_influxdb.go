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
	"syscall"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/ui"
)

func getLoginInfluxDBCommand(logger logrus.FieldLogger) *cobra.Command {
	// loginInfluxDBCommand represents the 'login influxdb' command
	loginInfluxDBCommand := &cobra.Command{
		Use:   "influxdb [uri]",
		Short: "Authenticate with InfluxDB",
		Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fs := afero.NewOsFs()
			config, configPath, err := readDiskConfig(fs)
			if err != nil {
				return err
			}

			conf := influxdb.NewConfig().Apply(config.Collectors.InfluxDB)
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
			if !terminal.IsTerminal(int(syscall.Stdin)) { // nolint: unconvert
				logger.Warn("Stdin is not a terminal, falling back to plain text input")
			}
			vals, err := form.Run(os.Stdin, stdout)
			if err != nil {
				return err
			}

			dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				DecodeHook: types.NullDecoder,
				Result:     &conf,
			})
			if err != nil {
				return err
			}

			if err = dec.Decode(vals); err != nil {
				return err
			}

			coll, err := influxdb.New(logger, conf)
			if err != nil {
				return err
			}
			if _, _, err := coll.Client.Ping(10 * time.Second); err != nil {
				return err
			}

			config.Collectors.InfluxDB = conf
			return writeDiskConfig(fs, configPath, config)
		},
	}
	return loginInfluxDBCommand
}
