package master

import (
	log "github.com/Sirupsen/logrus"
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

func (c *Connector) Run() (chan string, <-chan error) {
	ch := make(chan string)
	errors := make(chan error)

	// Start a read loop
	go func() {
		log.Info("-> Connector Read Loop")
		msg, err := c.InSocket.Recv()
		if err != nil {
			errors <- err
		}
		ch <- string(msg)
		log.Info("<- Connector Read Loop")
	}()

	// // Start a write loop
	go func() {
		log.Info("-> Connector Write Loop")
		msg := <-ch
		if err := c.OutSocket.Send([]byte(msg)); err != nil {
			errors <- err
		}
		log.Info("<- Connector Write Loop")
	}()

	return ch, errors
}
