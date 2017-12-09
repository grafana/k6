package cmd

import (
	"os"

	"github.com/loadimpact/k6/ui"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
)

// loginCloudCommand represents the 'login cloud' command
var loginCloudCommand = &cobra.Command{
	Use:   "cloud",
	Short: "Authenticate with Load Impact",
	Long: `Authenticate with Load Impact.

This will set the default server used when just "-o cloud" is passed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, cdir, err := readDiskConfig()
		if err != nil {
			return err
		}

		conf := config.Collectors.Cloud
		form := ui.Form{
			Fields: []ui.Field{
				ui.StringField{
					Key:     "token",
					Label:   "API Token",
					Default: conf.Token,
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

		config.Collectors.Cloud = conf
		return writeDiskConfig(cdir, config)
	},
}

func init() {
	loginCmd.AddCommand(loginCloudCommand)
}
