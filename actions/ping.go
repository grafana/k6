package actions

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/worker"
	"time"
)

func init() {
	registry.RegisterCommand(cli.Command{
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
			common.MasterHostFlag,
			common.MasterPortFlag,
		},
	})
	registry.RegisterHandler(handlePing)
	registry.RegisterProcessor(processPing)
}

// Processes worker pings.
func processPing(w *worker.Worker, msg message.Message, out chan message.Message) bool {
	switch msg.Type {
	case "ping.ping":
		out <- message.NewToClient("ping.pong", msg.Fields)
		return true
	default:
		return false
	}
}

// Handles master pings.
func handlePing(m *master.Master, msg message.Message, out chan message.Message) bool {
	switch msg.Type {
	case "ping.ping":
		out <- message.NewToClient("ping.pong", msg.Fields)
		return true
	default:
		return false
	}
}

// Pings a master or specified workers.
func actionPing(c *cli.Context) {
	client, local := common.MustGetClient(c)
	if local && !c.Bool("local") {
		log.Fatal("You're about to ping an in-process system, which doesn't make a lot of sense. You probably want to specify --master=..., or use --local if this is actually what you want.")
	}

	in, out, errors := client.Connector.Run()

	msgTopic := message.MasterTopic
	if c.Bool("worker") {
		msgTopic = message.WorkerTopic
	}
	out <- message.Message{
		Topic: msgTopic,
		Type:  "ping.ping",
		Fields: message.Fields{
			"time": time.Now().Format("15:04:05 2006-01-02 MST"),
		},
	}

readLoop:
	for {
		select {
		case msg := <-in:
			switch msg.Type {
			case "ping.pong":
				log.WithFields(log.Fields{
					"time": msg.Fields["time"],
				}).Info("Pong!")
				break readLoop
			}
		case err := <-errors:
			log.WithError(err).Error("Ping failed")
		}
	}
}
