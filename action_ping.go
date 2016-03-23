package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/worker"
	"time"
)

func init() {
	registerCommand(cli.Command{
		Name:   "ping",
		Usage:  "Tests master connectivity",
		Action: actionPing,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "worker",
				Usage: "Pings a worker instead of the master",
			},
			common.MasterHostFlag,
			common.MasterPortFlag,
		},
	})
	registerHandler(handlePing)
	registerProcessor(processPing)
}

func processPing(w *worker.Worker, msg message.Message, out chan message.Message) bool {
	switch msg.Type {
	case "ping.ping":
		out <- message.NewToClient("ping.pong", msg.Body)
		return true
	default:
		return false
	}
}

func handlePing(m *master.Master, msg message.Message, out chan message.Message) bool {
	switch msg.Type {
	case "ping.ping":
		out <- message.NewToClient("ping.pong", msg.Body)
		return true
	default:
		return false
	}
}

func actionPing(c *cli.Context) {
	client, err := client.New("tcp://127.0.0.1:9595", "tcp://127.0.0.1:9596")
	if err != nil {
		log.WithError(err).Fatal("Couldn't create a client")
	}
	in, out, errors := client.Connector.Run()

	msgTopic := message.MasterTopic
	if c.Bool("worker") {
		msgTopic = message.WorkerTopic
	}
	out <- message.Message{
		Topic: msgTopic,
		Type:  "ping.ping",
		Body:  time.Now().Format("15:04:05 2006-01-02 MST"),
	}

readLoop:
	for {
		select {
		case msg := <-in:
			switch msg.Type {
			case "ping.pong":
				log.WithFields(log.Fields{
					"body": msg.Body,
				}).Info("Pong!")
				break readLoop
			}
		case err := <-errors:
			log.WithError(err).Error("Ping failed")
		}
	}
}
