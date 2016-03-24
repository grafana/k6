package actions

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"github.com/loadimpact/speedboat/worker"
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
	registry.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &RunProcessor{}
	})
}

type RunProcessor struct{}

func (p *RunProcessor) Process(msg message.Message) <-chan message.Message {
	ch := make(chan message.Message)

	go func() {
		defer func() {
			ch <- message.NewToClient("run.end", message.Fields{})
			close(ch)
		}()

		switch msg.Type {
		case "run.run":
			filename := msg.Fields["filename"].(string)
			src := msg.Fields["src"].(string)
			vus := int(msg.Fields["vus"].(float64))
			duration := time.Duration(msg.Fields["duration"].(float64)) * time.Millisecond

			log.WithFields(log.Fields{
				"filename": filename,
				"vus":      vus,
				"duration": duration,
			}).Debug("Running script")

			var r runner.Runner = nil

			r, err := js.New()
			if err != nil {
				ch <- message.NewToClient("run.error", message.Fields{"error": err})
				break
			}

			err = r.Load(filename, src)
			if err != nil {
				ch <- message.NewToClient("run.error", message.Fields{"error": err})
				break
			}

			for res := range runner.Run(r, vus, duration) {
				switch res := res.(type) {
				case runner.LogEntry:
					ch <- message.NewToClient("run.log", message.Fields{
						"text": res.Text,
					})
				case runner.Metric:
					ch <- message.NewToClient("run.metric", message.Fields{
						"start":    res.Start,
						"duration": res.Duration,
					})
				case error:
					ch <- message.NewToClient("run.error", message.Fields{
						"error": res.Error(),
					})
				}
			}
		}
	}()

	return ch
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
