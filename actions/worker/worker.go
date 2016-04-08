package actions

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/common"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	desc := "A worker executes distributed tasks, and reports back to its master."

	client.RegisterCommand(cli.Command{
		Name:        "worker",
		Usage:       "Runs a worker server for distributed tests",
		Description: desc,
		Flags: []cli.Flag{
			common.MasterHostFlag,
			common.MasterPortFlag,
		},
		Action: actionWorker,
	})
}

// Runs a standalone worker.
func actionWorker(c *cli.Context) {
	inAddr, outAddr, local := common.ParseMasterParams(c)

	// Running a standalone worker without a master doesn't make any sense
	if local {
		log.Fatal("No master specified")
	}

	w, err := worker.New(inAddr, outAddr)
	if err != nil {
		log.WithError(err).Fatal("Failed to start worker")
	}

	log.WithFields(log.Fields{
		"master": inAddr,
	}).Info("Worker running")

	w.Processors = worker.GlobalProcessors
	w.Run()
}
