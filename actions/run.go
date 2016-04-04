package actions

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"io/ioutil"
	"path"
)

func init() {
	registry.RegisterCommand(cli.Command{
		Name:   "run",
		Usage:  "Runs a load test",
		Action: actionRun,
		Flags: []cli.Flag{
			common.MasterHostFlag,
			common.MasterPortFlag,
			cli.StringFlag{
				Name:  "script, s",
				Usage: "Script file to run",
			},
			cli.IntFlag{
				Name:  "vus, u",
				Usage: "Virtual Users to simulate",
				Value: 2,
			},
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "Duration of the test",
				Value: "10s",
			},
		},
	})
}

func actionRun(c *cli.Context) {
	client, _ := common.MustGetClient(c)
	in, out, errs := client.Run()

	filename := c.Args()[0]
	conf := loadtest.NewConfig()
	if len(c.Args()) > 0 {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			log.WithError(err).Fatal("Couldn't read test file")
		}

		loadtest.ParseConfig(data, &conf)
	}

	if c.IsSet("script") {
		conf.Script = c.String("script")
	}
	if c.IsSet("duration") {
		conf.Duration = c.String("duration")
	}
	if c.IsSet("vus") {
		conf.VUs = c.Int("vus")
	}

	log.WithField("conf", conf).Info("Config")
	test, err := conf.Compile()
	if err != nil {
		log.WithError(err).Fatal("Invalid test")
	}
	log.WithField("test", test).Info("Test")

	if err = test.Load(path.Dir(filename)); err != nil {
		log.WithError(err).Fatal("Couldn't load script")
	}

	in, out, errs = test.Run(in, out, errs)
	sequencer := runner.NewSequencer()
runLoop:
	for {
		select {
		case msg, ok := <-in:
			// ok is false if in is closed
			if !ok {
				break runLoop
			}

			switch msg.Type {
			case "test.log":
				entry := runner.LogEntry{}
				if err := msg.Take(&entry); err != nil {
					log.WithError(err).Error("Couldn't decode log entry")
					break
				}
				log.WithFields(log.Fields{
					"text": entry.Text,
				}).Info("Test Log")
			case "test.metric":
				metric := runner.Metric{}
				if err := msg.Take(&metric); err != nil {
					log.WithError(err).Error("Couldn't decode metric")
					break
				}

				log.WithFields(log.Fields{
					"start":    metric.Start,
					"duration": metric.Duration,
				}).Debug("Test Metric")

				sequencer.Add(metric)
			case "error":
				var text string
				if err := msg.Take(&text); err != nil {
					log.WithError(err).Error("Failed to decode error?!")
				} else {
					log.WithError(errors.New(text)).Error("Script Error")
				}
			}
		case err := <-errs:
			log.WithError(err).Error("Error")
		}
	}

	stats := sequencer.Stats()
	log.WithField("count", sequencer.Count()).Info("Results")
	log.WithFields(log.Fields{
		"min": stats.Duration.Min,
		"max": stats.Duration.Max,
		"avg": stats.Duration.Avg,
		"med": stats.Duration.Med,
	}).Info("Duration")
}
