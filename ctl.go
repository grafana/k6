package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/api"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
	"os"
	"strconv"
)

var commandStatus = cli.Command{
	Name:      "status",
	Usage:     "Looks up the status of a running test",
	ArgsUsage: " ",
	Action:    actionStatus,
}

var commandScale = cli.Command{
	Name:      "scale",
	Usage:     "Scales a running test",
	ArgsUsage: "vus",
	Action:    actionScale,
}

var commandAbort = cli.Command{
	Name:      "abort",
	Usage:     "Aborts a running test",
	ArgsUsage: " ",
	Action:    actionAbort,
}

func actionStatus(cc *cli.Context) error {
	client, err := api.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.Status()
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}

	bytes, err := yaml.Marshal(status)
	if err != nil {
		log.WithError(err).Error("Serialization Error")
		return err
	}
	_, _ = os.Stdout.Write(bytes)

	return nil
}

func actionScale(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	vus, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}

	client, err := api.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	if err := client.Scale(vus); err != nil {
		log.WithError(err).Error("Error")
	}
	return nil
}

func actionAbort(cc *cli.Context) error {
	client, err := api.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	if err := client.Abort(); err != nil {
		log.WithError(err).Error("Error")
	}
	return nil
}
