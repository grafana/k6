package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/client"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
	"os"
	"strconv"
)

func actionStatus(cc *cli.Context) error {
	client, err := client.New(cc.GlobalString("address"))
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

	client, err := client.New(cc.GlobalString("address"))
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
	client, err := client.New(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	if err := client.Abort(); err != nil {
		log.WithError(err).Error("Error")
	}
	return nil
}
