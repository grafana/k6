package run

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/runner"
	"io/ioutil"
	"path"
	"time"
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

func parseMetric(msg message.Message) (m runner.Metric, err error) {
	duration, ok := msg.Fields["duration"].(float64)
	if !ok {
		return m, errors.New("Duration is not a float64")
	}

	m.Duration = time.Duration(int64(duration))
	return m, nil
}

func actionRun(c *cli.Context) {
	client, _ := common.MustGetClient(c)
	in, out, errors := client.Run()

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

	in, out, errors = test.Run(in, out, errors)
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
			case "run.log":
				log.WithFields(log.Fields{
					"text": msg.Fields["text"],
				}).Info("Test Log")
			case "run.metric":
				m, err := parseMetric(msg)
				if err != nil {
					log.WithError(err).Error("Couldn't parse metric")
					break
				}

				log.WithFields(log.Fields{
					"start":    m.Start,
					"duration": m.Duration,
				}).Debug("Test Metric")

				sequencer.Add(m)
			case "run.error":
				log.WithFields(log.Fields{
					"error": msg.Fields["error"],
				}).Error("Script Error")
			}
		case err := <-errors:
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
