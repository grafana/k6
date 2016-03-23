package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	desc := "A worker executes distributed tasks, and reports back to its master."

	registerCommand(cli.Command{
		Name:        "worker",
		Usage:       "Runs a worker server for distributed tests",
		Description: desc,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "host, h",
				Usage: "Host for the master process",
				Value: "127.0.0.1",
			},
			cli.IntFlag{
				Name:  "port, p",
				Usage: "Base port for the master process",
				Value: 9595,
			},
		},
		Action: actionWorker,
	})
}

func actionWorker(c *cli.Context) {
	host := c.String("host")
	port := c.Int("port")

	inAddr := fmt.Sprintf("tcp://%s:%d", host, port)
	outAddr := fmt.Sprintf("tcp://%s:%d", host, port+1)
	worker, err := worker.New(inAddr, outAddr)
	if err != nil {
		log.WithError(err).Fatal("Couldn't start worker")
	}

	log.WithFields(log.Fields{
		"host": host,
		"pub":  port,
		"sub":  port + 1,
	}).Info("Worker running")
	worker.Processors = globalProcessors
	worker.Run()
}
