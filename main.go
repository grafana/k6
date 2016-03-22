package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/client"
	"github.com/loadimpact/speedboat/master"
)

func main() {
	master, err := master.New("inproc://master.pub", "inproc://master.sub")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to start master")
	}
	go master.Run()

	client, err := client.New("inproc://master.pub", "inproc://master.sub")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to start client")
	}
	client.Run()

	log.WithFields(log.Fields{
		"thing": "aaaa",
	}).Info("Is this working??")
}
