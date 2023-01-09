package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui"
)

//nolint:funlen,gocognit
func getCmdLoginCloud(globalState *globalState) *cobra.Command {
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
			currentDiskConf, err := readDiskConfig(globalState)
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
				currentJSONConfigRaw, globalState.envVars, "", nil)
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
				printToStdout(globalState, "  token reset\n")
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
				if !term.IsTerminal(int(syscall.Stdin)) { //nolint:unconvert
					globalState.logger.Warn("Stdin is not a terminal, falling back to plain text input")
				}
				var vals map[string]string
				vals, err = form.Run(globalState.stdIn, globalState.stdOut)
				if err != nil {
					return err
				}
				email := vals["Email"]
				password := vals["Password"]

				client := cloudapi.NewClient(
					globalState.logger,
					"",
					consolidatedCurrentConfig.Host.String,
					consts.Version,
					consolidatedCurrentConfig.Timeout.TimeDuration())

				var res *cloudapi.LoginResponse
				res, err = client.Login(email, password)
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
			if err := writeDiskConfig(globalState, currentDiskConf); err != nil {
				return err
			}

			if newCloudConf.Token.Valid {
				valueColor := getColor(globalState.flags.noColor || !globalState.stdOut.isTTY, color.FgCyan)
				if !globalState.flags.quiet {
					printToStdout(globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))
				}
				printToStdout(globalState, fmt.Sprintf(
					"Logged in successfully, token saved in %s\n", globalState.flags.configFilePath,
				))
			}
			return nil
		},
	}

	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset token")

	return loginCloudCommand
}
