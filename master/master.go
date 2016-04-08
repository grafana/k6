package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/message"
)

// A Master serves as a semi-intelligent message bus between clients and workers.
type Master struct {
	Connector  Connector
	Processors []func(*Master) Processor
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
	in, out := m.Connector.Run()
	pInstances := m.createProcessors()
	for msg := range in {
		log.WithFields(log.Fields{
			"type":    msg.Type,
			"topic":   msg.Topic,
			"payload": string(msg.Payload),
		}).Debug("Master Received")

		// If it's not intended for the master, rebroadcast
		if msg.Topic != message.MasterTopic {
			out <- msg
			continue
		}

		// Let master processors have a stab at them instead
		go func() {
			for m := range Process(pInstances, msg) {
				out <- m
			}
		}()
	}
}

func (m *Master) createProcessors() []Processor {
	pInstances := []Processor{}
	for _, fn := range m.Processors {
		pInstances = append(pInstances, fn(m))
	}
	return pInstances
}
