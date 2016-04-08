package client

import (
	// log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/comm"
)

// A Client controls load test execution.
type Client struct {
	Connector comm.Connector
}

// Creates a new Client, connecting to a Master listening on the given in/out addresses.
// The in/out addresses may be tcp:// or inproc:// addresses; see the documentation for
// mangos/nanomsg for more information.
func New(inAddr string, outAddr string) (c Client, err error) {
	c.Connector, err = comm.NewClientConnector(comm.ClientTopic, inAddr, outAddr)
	if err != nil {
		return c, err
	}

	return c, err
}

func (c *Client) Run() (<-chan comm.Message, chan comm.Message) {
	return c.Connector.Run()
}
