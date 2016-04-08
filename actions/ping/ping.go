package ping

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/comm"
	"github.com/loadimpact/speedboat/util"
	"time"
)

func init() {
	client.RegisterCommand(cli.Command{
		Name:   "ping",
		Usage:  "Tests master connectivity",
		Action: actionPing,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "worker",
				Usage: "Pings a worker instead of the master",
			},
			cli.BoolFlag{
				Name:  "local",
				Usage: "Allow pinging an inproc master/worker",
			},
			util.MasterHostFlag,
			util.MasterPortFlag,
		},
	})
}

// Pings a master or specified workers.
func actionPing(c *cli.Context) {
	ct, local := util.MustGetClient(c)
	if local && !c.Bool("local") {
		log.Fatal("You're about to ping an in-process system, which doesn't make a lot of sense. You probably want to specify --master=..., or use --local if this is actually what you want.")
	}

	in, out := ct.Connector.Run()

	topic := comm.MasterTopic
	if c.Bool("worker") {
		topic = comm.WorkerTopic
	}
	out <- comm.To(topic, "ping.ping").With(PingMessage{
		Time: time.Now(),
	})

readLoop:
	for msg := range in {
		switch msg.Type {
		case "ping.pong":
			data := PingMessage{}
			if err := msg.Take(&data); err != nil {
				log.WithError(err).Error("Couldn't decode pong")
				break
			}
			log.WithField("time", data.Time).Info("Pong!")
			break readLoop
		}
	}
}
