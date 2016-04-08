package util

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
)

// Common flag for specifying a master host.
var MasterHostFlag = cli.StringFlag{
	Name:  "master, m",
	Usage: "Host for the master process",
}

// Common flag for specifying a master port.
var MasterPortFlag = cli.IntFlag{
	Name:  "port, p",
	Usage: "Base port for the master process",
	Value: 9595,
}

// Parses master-related commandline params (MasterHostFlag and MasterPortFlag).
// Returns an in-address and an out-address in nanomsg format, and whether or not the master in
// question refers to one running inside this process.
func ParseMasterParams(c *cli.Context) (inAddr, outAddr string, local bool) {
	switch {
	case c.IsSet("master"):
		host := c.String("master")
		port := c.Int("port")
		inAddr = fmt.Sprintf("tcp://%s:%d", host, port)
		outAddr = fmt.Sprintf("tcp://%s:%d", host, port+1)
		local = false
	default:
		inAddr = "inproc://master.pub"
		outAddr = "inproc://master.sub"
		local = true
	}
	return inAddr, outAddr, local
}

// MustGetClient returns a connected client, or terminates the program if this fails. It will run
// a local master and worker (using RunLocalMaster and RunLocalWorker) if necessary. This is a
// helper function meant to cut down on the boilerplate needed to develop a new command.
func MustGetClient(c *cli.Context) (cl client.Client, local bool) {
	inAddr, outAddr, local := ParseMasterParams(c)

	// If we're running locally, ensure a local master and worker are running
	if local {
		if err := RunLocalMaster(inAddr, outAddr); err != nil {
			log.WithError(err).Fatal("Failed to start local master")
		}
		if err := RunLocalWorker(inAddr, outAddr); err != nil {
			log.WithError(err).Fatal("Failed to start local worker")
		}
	}

	cl, err := client.New(inAddr, outAddr)
	if err != nil {
		log.WithError(err).Fatal("Failed to start a client")
	}
	return cl, local
}
