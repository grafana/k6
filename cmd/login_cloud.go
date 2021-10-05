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
	"errors"
	"os"
	"syscall"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui"
)

//nolint:funlen
func getLoginCloudCommand(logger logrus.FieldLogger) *cobra.Command {
	// loginCloudCommand represents the 'login cloud' command
	loginCloudCommand := &cobra.Command{
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

			currentDiskConf, configPath, err := readDiskConfig(fs)
			if err != nil {
				return err
			}

			currentJSONConfig := cloudapi.Config{}
			currentJSONConfigRaw := currentDiskConf.Collectors["cloud"]
			if currentJSONConfigRaw != nil {
				// We only want to modify this config, see comment below
				if jsonerr := json.Unmarshal(currentJSONConfigRaw, &currentJSONConfig); jsonerr != nil {
					return jsonerr
				}
			}

			// We want to use this fully consolidated config for things like
			// host addresses, so users can overwrite them with env vars.
			consolidatedCurrentConfig, err := cloudapi.GetConsolidatedConfig(
				currentJSONConfigRaw, buildEnvMap(os.Environ()), "", nil)
			if err != nil {
				return err
			}
			// But we don't want to save them back to the JSON file, we only
			// want to save what already existed there and the login details.
			newCloudConf := currentJSONConfig

			show := getNullBool(cmd.Flags(), "show")
			reset := getNullBool(cmd.Flags(), "reset")
			token := getNullString(cmd.Flags(), "token")
			switch {
			case reset.Valid:
				newCloudConf.Token = null.StringFromPtr(nil)
				fprintf(stdout, "  token reset\n")
			case show.Bool:
			case token.Valid:
				newCloudConf.Token = token
			default:
				form := ui.Form{
					Fields: []ui.Field{
						ui.StringField{
							Key:   "Email",
							Label: "Email",
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
				email := vals["Email"].(string)
				password := vals["Password"].(string)

				client := cloudapi.NewClient(logger, "", consolidatedCurrentConfig.Host.String, consts.Version)
				res, err := client.Login(email, password)
				if err != nil {
					return err
				}

				if res.Token == "" {
					return errors.New(`your account has no API token, please generate one at https://app.k6.io/account/api-token`)
				}

				newCloudConf.Token = null.StringFrom(res.Token)
			}

			if currentDiskConf.Collectors == nil {
				currentDiskConf.Collectors = make(map[string]json.RawMessage)
			}
			currentDiskConf.Collectors["cloud"], err = json.Marshal(newCloudConf)
			if err != nil {
				return err
			}
			if err := writeDiskConfig(fs, configPath, currentDiskConf); err != nil {
				return err
			}

			if newCloudConf.Token.Valid {
				valueColor := getColor(noColor || !stdoutTTY, color.FgCyan)
				fprintf(stdout, "  token: %s\n", valueColor.Sprint(newCloudConf.Token.String))
			}
			return nil
		},
	}

	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset token")

	return loginCloudCommand
}
