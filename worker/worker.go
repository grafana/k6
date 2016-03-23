package worker

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/master"
)

// A Worker executes distributed tasks, communicating over a Master.
type Worker struct {
	Connector  master.Connector
	Processors []func(*Worker, master.Message, chan master.Message) bool
}

// Creates a new Worker, connecting to a master listening on the given in/out addresses.
func New(inAddr string, outAddr string) (w Worker, err error) {
	w.Connector, err = master.NewClientConnector(inAddr, outAddr)
	if err != nil {
		return w, err
	}

	return w, nil
}

// Runs the main loop for a worker.
func (w *Worker) Run() {
	in, out, errors := w.Connector.Run()
	for {
		select {
		case msg := <-in:
			log.WithFields(log.Fields{
				"type": msg.Type,
				"body": msg.Body,
			}).Info("Message Received")

			// Call handlers until we find one that responds
			for _, processor := range w.Processors {
				if processor(w, msg, out) {
					break
				}
			}

		case err := <-errors:
			log.WithError(err).Error("Error")
		}
	}
}
