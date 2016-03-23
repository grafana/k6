package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/worker"
	"time"
)

func init() {
	registerCommand(cli.Command{
		Name:   "ping",
		Usage:  "Test command, will be removed",
		Action: actionPing,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "worker",
				Usage: "Pings a worker instead of the master",
			},
		},
	})
	registerHandler(handlePing)
	registerProcessor(processPing)
}

func processPing(w *worker.Worker, msg master.Message, out chan master.Message) bool {
	switch msg.Type {
	case "ping.worker.ping":
		out <- master.Message{
			Type: "ping.worker.pong",
			Body: msg.Body,
		}
		return true
	default:
		return false
	}
}

func handlePing(m *master.Master, msg master.Message, out chan master.Message) bool {
	switch msg.Type {
	case "ping.ping":
		out <- master.Message{
			Type: "ping.pong",
			Body: msg.Body,
		}
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

	// Send a bunch of noise to filter through
	out <- master.Message{Type: "ping.noise"}
	out <- master.Message{Type: "ping.noise"}

	// Send a ping message, target should reply with a pong
	msgType := "ping.ping"
	if c.Bool("worker") {
		msgType = "ping.worker.ping"
	}
	out <- master.Message{
		Type: msgType,
		Body: time.Now().Format("15:04:05 2006-01-02 MST"),
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
			case "ping.worker.pong":
				log.WithFields(log.Fields{
					"body": msg.Body,
				}).Info("Worker Pong!")
				break readLoop
			}
		case err := <-errors:
			log.WithError(err).Error("Ping failed")
		}
	}
}
