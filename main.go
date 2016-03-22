package main

import (
	log "github.com/Sirupsen/logrus"
)

func main() {
	log.WithFields(log.Fields{
		"thing": "aaaa",
	}).Info("Is this working??")
}
