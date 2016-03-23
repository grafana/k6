package actions

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/master"
)

func init() {
	desc := "A master server acts as a message bus, between a clients and workers.\n" +
		"\n" +
		"The master works by opening TWO ports: a PUB port and a SUB port. Your firewall " +
		"must allow access to both of these, or clients will not be able to communicate " +
		"properly with the master."

	registry.RegisterCommand(cli.Command{
		Name:        "master",
		Usage:       "Runs a master server for distributed tests",
		Description: desc,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "host, h",
				Usage: "Listen on the given address",
				Value: "127.0.0.1",
			},
			cli.IntFlag{
				Name:  "port, p",
				Usage: "Listen on this port (PUB) + the next (SUB)",
				Value: 9595,
			},
		},
		Action: actionMaster,
	})
}

// Runs a master.
func actionMaster(c *cli.Context) {
	host := c.String("host")
	port := c.Int("port")

	outAddr := fmt.Sprintf("tcp://%s:%d", host, port)
	inAddr := fmt.Sprintf("tcp://%s:%d", host, port+1)
	master, err := master.New(outAddr, inAddr)
	if err != nil {
		log.WithError(err).Fatal("Couldn't start master")
	}

	log.WithFields(log.Fields{
		"host": host,
		"pub":  port,
		"sub":  port + 1,
	}).Info("Master running")
	master.Processors = registry.GlobalMasterProcessors
	master.Run()
}
