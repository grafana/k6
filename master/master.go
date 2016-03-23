package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/message"
)

// A Master serves as a semi-intelligent message bus between clients and workers.
type Master struct {
	Connector Connector
	Handlers  []func(*Master, message.Message, chan message.Message) bool
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
	in, out, errors := m.Connector.Run()
	for {
		select {
		case msg := <-in:
			log.WithFields(log.Fields{
				"type": msg.Type,
				"body": msg.Body,
			}).Info("Message Received")

			// If it's not intended for the master, rebroadcast
			if msg.Topic != message.MasterTopic {
				out <- msg
				break
			}

			// Call handlers until we find one that responds
			for _, handler := range m.Handlers {
				if handler(m, msg, out) {
					break
				}
			}
		case err := <-errors:
			log.WithError(err).Error("Error")
		}
	}
}
