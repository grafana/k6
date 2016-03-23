package worker

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
)

// A Worker executes distributed tasks, communicating over a Master.
type Worker struct {
	Connector  master.Connector
	Processors []func(*Worker) master.Processor

	pInstances []master.Processor
}

// Creates a new Worker, connecting to a master listening on the given in/out addresses.
func New(inAddr string, outAddr string) (w Worker, err error) {
	w.Connector, err = master.NewClientConnector(message.WorkerTopic, inAddr, outAddr)
	if err != nil {
		return w, err
	}

	return w, nil
}

// Runs the main loop for a worker.
func (w *Worker) Run() {
	w.createProcessors()
	in, out, errors := w.Connector.Run()
	for {
		select {
		case msg := <-in:
			log.WithFields(log.Fields{
				"type":   msg.Type,
				"fields": msg.Fields,
			}).Debug("Worker Received")

			for m := range master.Process(w.pInstances, msg) {
				out <- m
			}

		case err := <-errors:
			log.WithError(err).Error("Error")
		}
	}
}

func (w *Worker) createProcessors() {
	w.pInstances = []master.Processor{}
	for _, fn := range w.Processors {
		w.pInstances = append(w.pInstances, fn(w))
	}
}
