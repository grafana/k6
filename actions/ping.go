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
	registry.RegisterMasterProcessor(func(*master.Master) master.Processor {
		return &PingMasterProcessor{}
	})
	registry.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &PingProcessor{}
	})
}

// Processes worker pings.
type PingProcessor struct{}

func (*PingProcessor) Process(msg message.Message) <-chan message.Message {
	out := make(chan message.Message)

	go func() {
		defer close(out)
		switch msg.Type {
		case "ping.ping":
			out <- message.NewToClient("ping.pong", msg.Fields)
		}
	}()

	return out
}

// Processes master pings.
type PingMasterProcessor struct{}

func (*PingMasterProcessor) Process(msg message.Message) <-chan message.Message {
	out := make(chan message.Message)

	go func() {
		defer close(out)
		switch msg.Type {
		case "ping.ping":
			out <- message.NewToClient("ping.pong", msg.Fields)
		}
	}()

	return out
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
