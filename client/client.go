package client

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/master"
)

// A Client controls load test execution.
type Client struct {
	Connector master.Connector
}

// Creates a new Client, connecting to a Master listening on the given in/out addresses.
// The in/out addresses may be tcp:// or inproc:// addresses; see the documentation for
// mangos/nanomsg for more information.
func New(inAddr string, outAddr string) (c Client, err error) {
	c.Connector, err = master.NewClientConnector(inAddr, outAddr)
	if err != nil {
		return c, err
	}

	return c, err
}

// Runs the main loop for a client. This is probably going to go away.
func (c *Client) Run() {
	ch, errors := c.Connector.Run()
	for {
		select {
		case msg := <-ch:
			log.WithFields(log.Fields{
				"msg": msg,
			}).Info("Client: Message received")
		case err := <-errors:
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Client: Error receiving")
		}
	}
}
