package run

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/message"
	"io/ioutil"
	"time"
)

func init() {
	registry.RegisterCommand(cli.Command{
		Name:   "run",
		Usage:  "Runs a load test",
		Action: actionRun,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "script, s",
				Usage: "Script file to run",
			},
			cli.IntFlag{
				Name:  "vus, u",
				Usage: "Virtual Users to simulate",
				Value: 2,
			},
			cli.DurationFlag{
				Name:  "duration, d",
				Usage: "Duration of the test",
				Value: time.Duration(10) * time.Second,
			},
		},
	})
}

func actionRun(c *cli.Context) {
	client, _ := common.MustGetClient(c)
	in, out, errors := client.Connector.Run()

	if !c.IsSet("script") {
		log.Fatal("No script file specified!")
	}

	filename := c.String("script")
	srcb, err := ioutil.ReadFile(filename)
	src := string(srcb)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"filename": filename,
		}).Fatal("Couldn't read script")
	}

	out <- message.NewToWorker("run.run", message.Fields{
		"filename": c.String("script"),
		"src":      src,
		"vus":      c.Int("vus"),
		"duration": int64(c.Duration("duration")) / int64(time.Millisecond),
	})

readLoop:
	for {
		select {
		case msg := <-in:
			switch msg.Type {
			case "run.log":
				log.WithFields(log.Fields{
					"text": msg.Fields["text"],
				}).Info("Test Log")
			case "run.metric":
				log.WithFields(log.Fields{
					"start":    msg.Fields["start"],
					"duration": msg.Fields["duration"],
				}).Info("Test Metric")
			case "run.error":
				log.WithFields(log.Fields{
					"error": msg.Fields["error"],
				}).Error("Script Error")
			case "run.end":
				log.Info("-- Test End --")
				break readLoop
			}
		case err := <-errors:
			log.WithError(err).Error("Ping failed")
		}
	}
}
