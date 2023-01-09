package cmd

import (
	"encoding/json"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/output/influxdb"
	"go.k6.io/k6/ui"
)

//nolint:funlen
func getCmdLoginInfluxDB(globalState *globalState) *cobra.Command {
	// loginInfluxDBCommand represents the 'login influxdb' command
	loginInfluxDBCommand := &cobra.Command{
		Use:   "influxdb [uri]",
		Short: "Authenticate with InfluxDB",
		Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := readDiskConfig(globalState)
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
			if !term.IsTerminal(int(syscall.Stdin)) { //nolint:unconvert
				globalState.logger.Warn("Stdin is not a terminal, falling back to plain text input")
			}
			vals, err := form.Run(globalState.stdIn, globalState.stdOut)
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
			return writeDiskConfig(globalState, config)
		},
	}
	return loginInfluxDBCommand
}
