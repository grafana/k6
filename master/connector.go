package master

import (
	// log "github.com/Sirupsen/logrus"
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pub"
	"github.com/go-mangos/mangos/protocol/sub"
	"github.com/go-mangos/mangos/transport/inproc"
	"github.com/go-mangos/mangos/transport/tcp"
)

// A bidirectional pub/sub connector, used to connect to a master.
type Connector struct {
	InSocket  mangos.Socket
	OutSocket mangos.Socket
}

// Creates a bare, unconnected connector.
func NewBareConnector() (conn Connector, err error) {
	if conn.OutSocket, err = pub.NewSocket(); err != nil {
		return conn, err
	}

	if conn.InSocket, err = sub.NewSocket(); err != nil {
		return conn, err
	}

	return conn, nil
}

func NewClientConnector(inAddr string, outAddr string) (conn Connector, err error) {
	if conn, err = NewBareConnector(); err != nil {
		return conn, err
	}
	if err = setupAndDial(conn.InSocket, inAddr); err != nil {
		return conn, err
	}
	if err = setupAndDial(conn.OutSocket, outAddr); err != nil {
		return conn, err
	}

	err = conn.InSocket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return conn, err
	}

	return conn, nil
}

func NewServerConnector(outAddr string, inAddr string) (conn Connector, err error) {
	if conn, err = NewBareConnector(); err != nil {
		return conn, err
	}
	if err = setupAndListen(conn.OutSocket, outAddr); err != nil {
		return conn, err
	}
	if err = setupAndListen(conn.InSocket, inAddr); err != nil {
		return conn, err
	}

	err = conn.InSocket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return conn, err
	}

	return conn, nil
}

func setupSocket(sock mangos.Socket) {
	sock.AddTransport(inproc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
}

func setupAndListen(sock mangos.Socket, addr string) error {
	setupSocket(sock)
	if err := sock.Listen(addr); err != nil {
		return err
	}
	return nil
}

func setupAndDial(sock mangos.Socket, addr string) error {
	setupSocket(sock)
	if err := sock.Dial(addr); err != nil {
		return err
	}
	return nil
}

// Provides a channel-based interface around the underlying socket API.
func (c *Connector) Run() (<-chan Message, chan Message, <-chan error) {
	errors := make(chan error)
	in := make(chan Message)
	out := make(chan Message)

	// Read incoming messages
	go func() {
		for {
			msg, err := c.Read()
			if err != nil {
				errors <- err
				continue
			}
			in <- msg
		}
	}()

	// Write outgoing messages
	go func() {
		for {
			msg := <-out
			err := c.Write(msg)
			if err != nil {
				errors <- err
				continue
			}
		}
	}()

	return in, out, errors
}

func (c *Connector) Read() (msg Message, err error) {
	data, err := c.InSocket.Recv()
	if err != nil {
		return msg, err
	}
	msg, err = DecodeMessage(data)
	return msg, nil
}

func (c *Connector) Write(msg Message) (err error) {
	body, err := msg.Encode()
	if err != nil {
		return err
	}
	err = c.OutSocket.Send(body)
	if err != nil {
		return err
	}
	return nil
}
