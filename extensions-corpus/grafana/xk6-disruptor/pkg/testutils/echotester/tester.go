// Package echotester provides utilities for testing against echo servers in integration tests.
package echotester

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Tester is a struct that can be used to test an echoserver is behaving as expected. Each tester uses and keeps
// a connection to an echoserver.
type Tester struct {
	conn   net.Conn
	reader *bufio.Reader
}

// NewTester opens a connection to the specified address and returns a tester that uses it.
func NewTester(address string) (*Tester, error) {
	t := &Tester{}

	var err error
	t.conn, err = net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", address, err)
	}

	t.reader = bufio.NewReader(t.conn)

	return t, nil
}

// Echo sends a message to the echoserver and checks the same message is received back.
func (t *Tester) Echo(waitTime time.Duration) error {
	const line = "I am a test!\n"

	_, err := t.conn.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("writing string: %w", err)
	}

	err = t.conn.SetReadDeadline(time.Now().Add(waitTime))
	if err != nil {
		return fmt.Errorf("setting read deadline: %w", err)
	}

	echoed, err := t.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading back: %w", err)
	}

	if echoed != line {
		return fmt.Errorf("echoed string %q does not match sent %q", echoed, line)
	}

	return nil
}

// Close closes the connection to the echoserver and tests the receiving end of the connection gets closed as well.
func (t *Tester) Close() error {
	err := t.conn.Close()
	if err != nil {
		return fmt.Errorf("closing connection: %w", err)
	}

	str, err := t.reader.ReadString('\n')
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("expected EOF after closing, got %w and read %q instead", err, str)
	}

	return nil
}
