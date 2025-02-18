package cmd

import (
	"encoding/json"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/output/influxdb"
	"go.k6.io/k6/internal/ui"
)

//nolint:funlen
func getCmdLoginInfluxDB(gs *state.GlobalState) *cobra.Command {
	// loginInfluxDBCommand represents the 'login influxdb' command
	loginInfluxDBCommand := &cobra.Command{
		Use:   "influxdb [uri]",
		Short: "Authenticate with InfluxDB",
		Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := migrateLegacyConfigFileIfAny(gs); err != nil {
				return err
			}

			config, err := readDiskConfig(gs)
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
				urlConf, err := influxdb.ParseURL(args[0])
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
				gs.Logger.Warn("Stdin is not a terminal, falling back to plain text input")
			}
			vals, err := form.Run(gs.Stdin, gs.Stdout)
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
			return writeDiskConfig(gs, config)
		},
	}
	return loginInfluxDBCommand
}
