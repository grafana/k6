package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"os"
)

func init() {
	registerCommand(cli.Command{
		Name:   "ping",
		Usage:  "Test command, will be removed",
		Action: actionPing,
	})
}

func actionPing(c *cli.Context) {
	client, err := client.New("tcp://127.0.0.1:9595", "tcp://127.0.0.1:9596")
	if err != nil {
		log.WithError(err).Fatal("Failed to ping")
	}

	go func() {
		client.Connector.Send("ping")
		msg := <-client.Connector.InChannel
		log.WithField("msg", msg).Info("Response")
		os.Exit(0)
	}()

	ch, errors := client.Connector.Run()
	select {
	case msg := <-ch:
		log.WithField("msg", msg).Info("Response")
	case err := <-errors:
		log.WithError(err).Error("Failed to ping master")
	}
}
