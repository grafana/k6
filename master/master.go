package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/message"
)

// A Master serves as a semi-intelligent message bus between clients and workers.
type Master struct {
	Connector  Connector
	Processors []func(*Master) Processor

	pInstances []Processor
}

// Creates a new Master, listening on the given in/out addresses.
// The in/out addresses may be tcp:// or inproc:// addresses.
// Note that positions of the in/out parameters are swapped compared to client.New(), to make
// `client.New(a, b)` connect to a master created with `master.New(a, b)`.
func New(outAddr string, inAddr string) (m Master, err error) {
	m.Connector, err = NewServerConnector(outAddr, inAddr)
	if err != nil {
		return m, err
	}

	return m, nil
}

// Runs the main loop for a master.
func (m *Master) Run() {
	m.createProcessors()
	in, out, errors := m.Connector.Run()
	for {
		select {
		case msg := <-in:
			log.WithFields(log.Fields{
				"type":   msg.Type,
				"fields": msg.Fields,
			}).Debug("Master Received")

			// If it's not intended for the master, rebroadcast
			if msg.Topic != message.MasterTopic {
				out <- msg
				break
			}

			// Let master processors have a stab at them instead
			for m := range Process(m.pInstances, msg) {
				out <- m
			}
		case err := <-errors:
			log.WithError(err).Error("Error")
		}
	}
}

func (m *Master) createProcessors() {
	m.pInstances = []Processor{}
	for _, fn := range m.Processors {
		m.pInstances = append(m.pInstances, fn(m))
	}
}
