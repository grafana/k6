package common

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/worker"
)

var MasterHostFlag = cli.StringFlag{
	Name:  "master, m",
	Usage: "Host for the master process",
}
var MasterPortFlag = cli.IntFlag{
	Name:  "port, p",
	Usage: "Base port for the master process",
	Value: 9595,
}

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

func RunLocalMaster(inAddr, outAddr string) error {
	m, err := master.New(inAddr, outAddr)
	if err != nil {
		return err
	}
	go m.Run()
	return nil
}

func RunLocalWorker(inAddr, outAddr string) error {
	w, err := worker.New(inAddr, outAddr)
	if err != nil {
		return err
	}
	go w.Run()
	return nil
}

func MustGetClient(c *cli.Context) client.Client {
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

	client, err := client.New(inAddr, outAddr)
	if err != nil {
		log.WithError(err).Fatal("Failed to start a client")
	}
	return client
}
