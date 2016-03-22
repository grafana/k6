package master

import (
	log "github.com/Sirupsen/logrus"
)

// A Master serves as a semi-intelligent message bus between clients and workers.
type Master struct {
	Connector Connector
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
	ch, errors := m.Connector.Run()
	for {
		select {
		case msg := <-ch:
			log.WithFields(log.Fields{
				"msg": msg,
			}).Info("Master: Message received")

			// Echo everything
			m.Connector.Send(msg)
		case err := <-errors:
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Master: Error")
		}
	}
}
