package cmd

import (
	"os"
	"time"

	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/ui"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
)

// loginInfluxDBCommand represents the resume command
var loginInfluxDBCommand = &cobra.Command{
	Use:   "influxdb [uri]",
	Short: "Authenticate with InfluxDB",
	Long: `Authenticate with InfluxDB.

This will set the default server used when just "-o influxdb" is passed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config, cdir, err := readDiskConfig()
		if err != nil {
			return err
		}

		var conf influxdb.Config
		if err := config.ConfigureCollector(collectorInfluxDB, &conf); err != nil {
			return err
		}
		if len(args) > 0 {
			if err := conf.UnmarshalText([]byte(args[0])); err != nil {
				return err
			}
		}
		if conf.Addr == "" {
			conf.Addr = "http://localhost:8086"
		}
		if conf.Database == "" {
			conf.Database = "k6"
		}

		form := ui.Form{
			Fields: []ui.Field{
				ui.StringField{
					Key:     "Addr",
					Label:   "Address",
					Default: conf.Addr,
				},
				ui.StringField{
					Key:     "Database",
					Label:   "Database",
					Default: conf.Database,
				},
				ui.StringField{
					Key:     "Username",
					Label:   "Username",
					Default: conf.Username,
				},
				ui.StringField{
					Key:     "Password",
					Label:   "Password",
					Default: conf.Password,
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

		coll, err := influxdb.New(conf)
		if err != nil {
			return err
		}
		if _, _, err := coll.Client.Ping(10 * time.Second); err != nil {
			return err
		}

		if err := config.SetCollectorConfig(collectorInfluxDB, conf); err != nil {
			return err
		}
		return writeDiskConfig(cdir, config)
	},
}

func init() {
	loginCmd.AddCommand(loginInfluxDBCommand)
}
