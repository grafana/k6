package master

import (
	log "github.com/Sirupsen/logrus"
)

// A Master serves as a semi-intelligent message bus between clients and workers.
type Master struct {
	Connector Connector
	Handlers  []func(*Master, Message, chan Message) bool
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

			// Call handlers until we find one that responds
			handled := false
			for _, handler := range m.Handlers {
				if handler(m, msg, out) {
					handled = true
					break
				}
			}

			// If it's not intended for the master, rebroadcast
			if !handled {
				out <- msg
			}
		case err := <-errors:
			log.WithError(err).Error("Error")
		}
	}
}
