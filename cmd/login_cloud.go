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

  # Set up the token.
  k6 login cloud -t YOUR_TOKEN

  # Ask for your Load Impact user email and password to automatically set up the token.
  k6 login cloud`[1:],

	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		printToken := func(conf cloud.Config) {
			fmt.Fprintf(stdout, "  token: %s\n", ui.ValueColor.Sprint(conf.Token))
		}

		config, cdir, err := readDiskConfig()
		if err != nil {
                        return err
                }

                flags := cmd.Flags()
                show, err := flags.GetBool("show")
                if err != nil {
                        return err
                }
                token, err := flags.GetString("token")
                if err != nil {
                        return err
                }

                conf := config.Collectors.Cloud

                switch {
                case show:
                        printToken(conf)
                        return nil
                case token != "":
                        conf.Token = token
                default:
                        printToken(conf)

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
                        response, err := client.Login(email, password)
                        if err != nil {
                                return err
                        }

                        if response.APIToken == "" {
                                return errors.New("Your account has no API token, please generate one: `https://app.loadimpact.com/account/token`.")
                        }

                        conf.Token = response.APIToken
                }

                config.Collectors.Cloud = conf
                if err := writeDiskConfig(cdir, config); err != nil {
                        return err
                }

                printToken(conf)
		return nil
	},
}

func init() {
	loginCmd.AddCommand(loginCloudCommand)
	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
}
