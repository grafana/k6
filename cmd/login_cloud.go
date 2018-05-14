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
	"fmt"
	"os"

	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// loginCloudCommand represents the 'login cloud' command
var loginCloudCommand = &cobra.Command{
	Use:   "cloud",
	Short: "Authenticate with Load Impact",
	Long: `Authenticate with Load Impact.

This will set the default token used when just "k6 run -o cloud" is passed.`,
	Example: `
  # Show the stored token.
  k6 login cloud -s

  # Store a token.
  k6 login cloud -t YOUR_TOKEN

  # Log in with an email/password.
  k6 login cloud`[1:],
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fs := afero.NewOsFs()
		config, cdir, err := readDiskConfig(fs)
		if err != nil {
			return err
		}

		show := getNullBool(cmd.Flags(), "show")
		token := getNullString(cmd.Flags(), "token")

		conf := cloud.NewConfig().Apply(config.Collectors.Cloud)

		switch {
		case show.Bool:
		case token.Valid:
			conf.Token = token
		default:
			form := ui.Form{
				Fields: []ui.Field{
					ui.StringField{
						Key:   "Email",
						Label: "Email",
					},
					ui.StringField{
						Key:   "Password",
						Label: "Password",
					},
				},
			}
			vals, err := form.Run(os.Stdin, stdout)
			if err != nil {
				return err
			}
			email := vals["Email"].(string)
			password := vals["Password"].(string)

			client := cloud.NewClient("", conf.Host.String, Version)
			res, err := client.Login(email, password)
			if err != nil {
				return err
			}

			if res.Token == "" {
				return errors.New(`Your account has no API token, please generate one: "https://app.loadimpact.com/account/token".`)
			}

			conf.Token = null.StringFrom(res.Token)
		}

		config.Collectors.Cloud = conf
		if err := writeDiskConfig(fs, cdir, config); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "  token: %s\n", ui.ValueColor.Sprint(conf.Token.String))
		return nil
	},
}

func init() {
	loginCmd.AddCommand(loginCloudCommand)
	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
}
