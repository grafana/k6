package cmd

import (
	"fmt"
	"os"

	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/ui"
	"github.com/pkg/errors"
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
                config, cdir, err := readDiskConfig()
                if err != nil {
                        return err
                }

                show := getNullBool(cmd.Flags(), "show")
                token := getNullString(cmd.Flags(), "token")

                conf := config.Collectors.Cloud

                switch {
                case show.Bool:
                case token.Valid:
                        conf.Token = token.String
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

                        client := cloud.NewClient("", conf.Host, Version)
                        res, err := client.Login(email, password)
                        if err != nil {
                                return err
                        }

                        if res.Token == "" {
                                return errors.New(`Your account has no API token, please generate one: "https://app.loadimpact.com/account/token".`)
                        }

                        conf.Token = res.Token
                }

                config.Collectors.Cloud = conf
                if err := writeDiskConfig(cdir, config); err != nil {
                        return err
                }

                fmt.Fprintf(stdout, "  token: %s\n", ui.ValueColor.Sprint(conf.Token))
                return nil
        },
}

func init() {
	loginCmd.AddCommand(loginCloudCommand)
	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
}
