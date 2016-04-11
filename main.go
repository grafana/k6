package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/loadtest"
	"io/ioutil"
	"os"
	"path"
)

func makeTest(c *cli.Context) (test loadtest.LoadTest, err error) {
	base := ""
	conf := loadtest.NewConfig()
	if len(c.Args()) > 0 {
		filename := c.Args()[0]
		base = path.Dir(filename)
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return test, err
		}

		loadtest.ParseConfig(data, &conf)
	}

	if c.IsSet("script") {
		conf.Script = c.String("script")
		base = ""
	}
	if c.IsSet("duration") {
		conf.Duration = c.String("duration")
	}
	if c.IsSet("vus") {
		conf.VUs = c.Int("vus")
	}

	test, err = conf.Compile()
	if err != nil {
		return test, err
	}

	if err = test.Load(base); err != nil {
		return test, err
	}

	return test, nil
}

func action(c *cli.Context) {
	test, err := makeTest(c)
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	log.WithField("test", test).Info("Test")
}

// Configure the global logger.
func configureLogging(c *cli.Context) {
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	// Free up -v and -h for our own flags
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help, ?"

	// Bootstrap using action-registered commandline flags
	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "0.0.1a1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "More verbose output",
		},
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}

// func main() {
// 	// Free up -v and -h for our own flags
// 	cli.VersionFlag.Name = "version"
// 	cli.HelpFlag.Name = "help, ?"

// 	// Bootstrap using action-registered commandline flags
// 	app := cli.NewApp()
// 	app.Name = "speedboat"
// 	app.Usage = "A next-generation load generator"
// 	app.Version = "0.0.1a1"
// 	app.Flags = []cli.Flag{
// 		cli.BoolFlag{
// 			Name:  "verbose, v",
// 			Usage: "More verbose output",
// 		},
// 	}
// 	app.Commands = client.GlobalCommands
// 	app.Before = func(c *cli.Context) error {
// 		configureLogging(c)
// 		return nil
// 	}
// 	app.Run(os.Args)
// }
