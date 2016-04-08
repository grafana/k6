package worker

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/comm"
	"github.com/loadimpact/speedboat/master"
)

// A Worker executes distributed tasks, communicating over a Master.
type Worker struct {
	Connector  comm.Connector
	Processors []func(*Worker) master.Processor
}

// Creates a new Worker, connecting to a master listening on the given in/out addresses.
func New(inAddr string, outAddr string) (w Worker, err error) {
	w.Connector, err = comm.NewClientConnector(comm.WorkerTopic, inAddr, outAddr)
	if err != nil {
		return w, err
	}

	return w, nil
}

// Runs the main loop for a worker.
func (w *Worker) Run() {
	in, out := w.Connector.Run()
	pInstances := w.createProcessors()
	for msg := range in {
		log.WithFields(log.Fields{
			"type":    msg.Type,
			"payload": string(msg.Payload),
		}).Debug("Worker Received")

		go func() {
			for m := range master.Process(pInstances, msg) {
				out <- m
			}
		}()
	}
}

func (w *Worker) createProcessors() []master.Processor {
	pInstances := []master.Processor{}
	for _, fn := range w.Processors {
		pInstances = append(pInstances, fn(w))
	}
	return pInstances
}
