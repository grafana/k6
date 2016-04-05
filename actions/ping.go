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
		return &PingProcessor{}
	})
	registry.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &PingProcessor{}
	})
}

// Processes pings, on both master and worker.
type PingProcessor struct{}

type PingMessage struct {
	Time time.Time
}

func (*PingProcessor) Process(msg message.Message) <-chan message.Message {
	out := make(chan message.Message)

	go func() {
		defer close(out)
		switch msg.Type {
		case "ping.ping":
			data := PingMessage{}
			if err := msg.Take(&data); err != nil {
				out <- message.ToClient("error").WithError(err)
				break
			}
			out <- message.ToClient("ping.pong").With(data)
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

	in, out := client.Connector.Run()

	topic := message.MasterTopic
	if c.Bool("worker") {
		topic = message.WorkerTopic
	}
	out <- message.To(topic, "ping.ping").With(PingMessage{
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
