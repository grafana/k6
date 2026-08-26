package redis

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode"
)

// RunT starts a new redis stub TCP server for a given test context.
// It registers the test cleanup after your test is done.
func RunT(t testing.TB) *StubServer {
	s := NewStubServer()
	if err := s.Start(false, nil); err != nil {
		t.Fatalf("could not start RedisStub; reason: %s", err)
	}

	t.Cleanup(s.Close)

	return s
}

// RunTSecure starts a new redis stub TLS server for a given test context.
// It registers the test cleanup after your test is done.
// Optionally, a client certificate in PEM format can be passed to enable TLS
// client authentication (mTLS).
func RunTSecure(t testing.TB, clientCert []byte) *StubServer {
	s := NewStubServer()
	if err := s.Start(true, clientCert); err != nil {
		t.Fatalf("could not start RedisStub; reason: %s", err)
	}

	t.Cleanup(s.Close)

	return s
}

// StubServer is a stub server emulating a Redis server.
//
// It implements a minimal redis server capable of handling
// redis request and response message in the standard RESP
// protocol format.
//
// It is intended to be used in tests. It listens on a random
// localhost port, and can be used to test client-server interactions.
// It parses incoming requests, and calls the user-defined handlers
// to simulate the server's behavior.
//
// It is not intended to be used in production.
type StubServer struct {
	sync.Mutex
	waitGroup sync.WaitGroup

	listener  net.Listener
	boundAddr *net.TCPAddr

	connections     map[net.Conn]struct{}
	connectionCount int

	handlers          map[string]func(*Connection, []string)
	processedCommands int
	commandsHistory   [][]string

	tlsCert []byte
	cert    *tls.Certificate

	// list of commands to not be recorded
	ignoredCommands []string
}

// NewStubServer instantiates a new RedisStub server.
func NewStubServer() *StubServer {
	return &StubServer{
		listener:        nil,
		boundAddr:       nil,
		connections:     map[net.Conn]struct{}{},
		handlers:        make(map[string]func(*Connection, []string)),
		ignoredCommands: []string{"CLIENT"}, // this is usually not interesting
	}
}

// Start the RedisStub server. If secure is true, a TLS server with a
// self-signed certificate will be started. Otherwise, an unencrypted TCP
// server will start.
// Optionally, a client certificate in PEM format can be passed to enable TLS
// client authentication (mTLS).
func (rs *StubServer) Start(secure bool, clientCert []byte) error {
	var (
		addr     = net.JoinHostPort("localhost", "0")
		listener net.Listener
	)
	if secure { //nolint: nestif
		// TODO: Generate the cert only once per test run and reuse it, instead
		// of once per StubServer start?
		cert, pkey, err := generateTLSCert()
		if err != nil {
			return err
		}
		rs.tlsCert = cert
		certPair, err := tls.X509KeyPair(cert, pkey)
		if err != nil {
			return err
		}
		rs.cert = &certPair
		config := &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
			Certificates:             []tls.Certificate{certPair},
		}
		if clientCert != nil {
			clientCertPool := x509.NewCertPool()
			clientCertPool.AppendCertsFromPEM(clientCert)
			config.ClientCAs = clientCertPool
			config.ClientAuth = tls.RequireAndVerifyClientCert
		}
		if listener, err = tls.Listen("tcp", addr, config); err != nil {
			return err
		}
	} else {
		var err error
		if listener, err = (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr); err != nil {
			return err
		}
	}

	rs.listener = listener

	// the provided addr string binds to port zero,
	// which leads to automatic port selection by the OS.
	// We need to get the actual port the OS bound us to.
	boundAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("could not get TCP address")
	}

	rs.boundAddr = boundAddr

	rs.waitGroup.Go(func() {
		rs.listenAndServe(listener)

		rs.Lock()
		for c := range rs.connections {
			c.Close() //nolint:errcheck
		}
		rs.Unlock()
	})

	// The redis-cli will always start a session by sending the
	// COMMAND message.
	rs.RegisterCommandHandler("COMMAND", func(c *Connection, _ []string) {
		c.WriteArray("OK")
	})

	// We register a default PING command handler
	rs.RegisterCommandHandler("PING", func(c *Connection, args []string) {
		if len(args) == 1 {
			c.WriteBulkString(args[0])
		} else {
			c.WriteSimpleString("PONG")
		}
	})

	return nil
}

// Close stops the RedisStub server.
func (rs *StubServer) Close() {
	rs.Lock()
	if rs.listener != nil {
		err := rs.listener.Close()
		if err != nil {
			// this is unrecoverable, so we panic.
			panic(err)
		}
	}
	rs.listener = nil
	rs.Unlock()

	rs.waitGroup.Wait()
}

// RegisterCommandHandler registers a handler for a redis command.
//
// The handler is called when the command is received. It gives access
// to a `*Connection` object, which can be used to send responses, using
// its `Write*` methods.
func (rs *StubServer) RegisterCommandHandler(command string, handler func(*Connection, []string)) {
	rs.Lock()
	defer rs.Unlock()
	rs.handlers[strings.ToUpper(command)] = handler
}

// Addr returns the address of the RedisStub server.
func (rs *StubServer) Addr() *net.TCPAddr {
	return rs.boundAddr
}

// HandledCommandsCount returns the total number of commands
// ran since the redis stub server started.
func (rs *StubServer) HandledCommandsCount() int {
	rs.Lock()
	defer rs.Unlock()
	return rs.processedCommands
}

// HandledConnectionsCount returns the number of established client
// connections since the redis stub server started.
func (rs *StubServer) HandledConnectionsCount() int {
	rs.Lock()
	defer rs.Unlock()
	return rs.connectionCount
}

// GotCommands returns the commands handled (ordered by arrival) by the redis server
// since it started.
func (rs *StubServer) GotCommands() [][]string {
	rs.Lock()
	defer rs.Unlock()
	return rs.commandsHistory
}

// TLSCertificate returns the TLS certificate used by the server.
func (rs *StubServer) TLSCertificate() []byte {
	return rs.tlsCert
}

// listenAndServe listens on the redis server's listener,
// and handles client connections.
func (rs *StubServer) listenAndServe(l net.Listener) {
	for {
		nc, err := l.Accept()
		if err != nil {
			return
		}

		rs.waitGroup.Add(1)
		rs.Lock()
		rs.connections[nc] = struct{}{}
		rs.connectionCount++
		rs.Unlock()

		go func() {
			defer rs.waitGroup.Done()
			defer nc.Close() //nolint:errcheck

			rs.handleConnection(nc)

			rs.Lock()
			delete(rs.connections, nc)
			rs.Unlock()
		}()
	}
}

// handleConnection handles a single redis client connection.
func (rs *StubServer) handleConnection(nc net.Conn) {
	connection := NewConnection(bufio.NewReader(nc), bufio.NewWriter(nc))

	for {
		command, args, err := connection.ParseRequest()
		if err != nil {
			connection.WriteError(ErrInvalidSyntax)
			return
		}

		if !slices.Contains(rs.ignoredCommands, command) {
			rs.Lock()
			request := append([]string{command}, args...)
			rs.commandsHistory = append(rs.commandsHistory, request)
			rs.Unlock()
		}

		rs.handleCommand(connection, command, args)
		connection.Flush()
	}
}

// ErrUnknownCommand is the error message returned when the server
// is unable to handle the provided command (because it is not registered).
var ErrUnknownCommand = errors.New("unknown command")

// HandleCommand handles the provided command and arguments.
//
// If the command is not known, it writes the ErrUnknownCommand
// error message to the Connection's writer.
func (rs *StubServer) handleCommand(c *Connection, cmd string, args []string) {
	rs.Lock()
	handlerFn, ok := rs.handlers[cmd]
	rs.Unlock()
	if !ok {
		c.WriteError(ErrUnknownCommand)
		return
	}

	rs.Lock()
	rs.processedCommands++
	rs.Unlock()

	handlerFn(c, args)
}

// Connection represents a client connection to the redis server.
type Connection struct {
	writer *bufio.Writer
	reader *bufio.Reader
	mutex  sync.Mutex
}

// NewConnection creates a new Connection from the provided
// buffered reader and writer (usually a net.Conn instance).
func NewConnection(r *bufio.Reader, w *bufio.Writer) *Connection {
	return &Connection{
		reader: r,
		writer: w,
	}
}

// ParseRequest parses a request from the Connection's reader.
// It returns the parsed command, arguments and any error.
func (c *Connection) ParseRequest() (string, []string, error) {
	return NewRESPRequestReader(c.reader).ReadCommand()
}

// Flush flushes the Connection's writer, effectively sending
// all buffered data to the client.
func (c *Connection) Flush() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.writer.Flush(); err != nil {
		// this is unrecoverable, so we panic.
		panic(err)
	}
}

// WriteSimpleString writes the provided value as a redis simple string message
// to the Connection's writer.
func (c *Connection) WriteSimpleString(s string) {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteSimpleString(s)
	})
}

// WriteError writes the provided error as a redis error message
// to the Connection's writer.
func (c *Connection) WriteError(err error) {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteError(err)
	})
}

// WriteInteger writes the provided integer as a redis integer message
// to the Connection's writer.
func (c *Connection) WriteInteger(i int) {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteInteger(i)
	})
}

// WriteBulkString writes the provided string as a redis bulk string message
// to the Connection's writer.
func (c *Connection) WriteBulkString(s string) {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteBulkString(s)
	})
}

// WriteArray writes the provided array of string as a redis array message
// to the Connection's writer.
func (c *Connection) WriteArray(arr ...string) {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteArray(arr...)
	})
}

// WriteNull writes a redis Null message to the Connection's writer.
func (c *Connection) WriteNull() {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteNull()
	})
}

// WriteOK is a helper method for writing the OK response to the
// Connection's writer.
func (c *Connection) WriteOK() {
	c.callFn(func(w *RESPResponseWriter) {
		w.WriteSimpleString("OK")
	})
}

// callFn calls the provided function in a locking manner.
//
// It is used to ensure that the Connection's writer is not
// modified while it is being written to.
func (c *Connection) callFn(fn func(*RESPResponseWriter)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	fn(&RESPResponseWriter{c.writer})
}

// RESPRequestReader is a RESP protocol request reader.
type RESPRequestReader struct {
	reader *bufio.Reader
}

// NewRESPRequestReader returns a new RESPRequestReader.
func NewRESPRequestReader(reader *bufio.Reader) *RESPRequestReader {
	return &RESPRequestReader{reader: reader}
}

// ReadCommand reads a RESP command from the reader, parses it, and
// returns the parsed command, args, and any potential error encountered.
func (rrr *RESPRequestReader) ReadCommand() (string, []string, error) {
	elements, err := scanArray(rrr.reader)
	if err != nil {
		return "", nil, err
	}

	if len(elements) < 1 {
		return "", nil, ErrInvalidSyntax
	}

	return strings.ToUpper(elements[0]), elements[1:], nil
}

// ErrInvalidSyntax is returned when a RESP protocol message
// is malformed and cannot be parsed.
var ErrInvalidSyntax = errors.New("invalid RESP protocol syntax")

// Prefix is a placeholder type for the prefix symbol of
// RESP response message.
type Prefix byte

// RESP protocol response type prefixes definitions
const (
	SimpleStringPrefix Prefix = '+'
	ErrorPrefix               = '-'
	IntegerPrefix             = ':'
	BulkStringPrefix          = '$'
	ArrayPrefix               = '*'
	UnknownPrefix
)

// RESPResponseWriter is a RESP protocol response writer.
type RESPResponseWriter struct {
	writer *bufio.Writer
}

// WriteSimpleString writes a redis inline string
func (rw *RESPResponseWriter) WriteSimpleString(s string) {
	_, _ = fmt.Fprintf(rw.writer, "+%s\r\n", inline(s))
}

// WriteError writes a redis 'Error'
func (rw *RESPResponseWriter) WriteError(err error) {
	_, _ = fmt.Fprintf(rw.writer, "-%s\r\n", inline(err.Error()))
}

// WriteInteger writes an integer
func (rw *RESPResponseWriter) WriteInteger(n int) {
	_, _ = fmt.Fprintf(rw.writer, ":%d\r\n", n)
}

// WriteBulkString writes a bulk string
func (rw *RESPResponseWriter) WriteBulkString(s string) {
	_, _ = fmt.Fprintf(rw.writer, "$%d\r\n%s\r\n", len(s), s)
}

// WriteArray writes a list of strings (bulk)
func (rw *RESPResponseWriter) WriteArray(strs ...string) {
	rw.writeLen(len(strs))
	for _, s := range strs {
		if s == "" || s == "nil" {
			rw.WriteNull()
			continue
		}

		rw.WriteBulkString(s)
	}
}

// WriteNull writes a redis Null element
func (rw *RESPResponseWriter) WriteNull() {
	_, _ = fmt.Fprintf(rw.writer, "$-1\r\n")
}

func (rw *RESPResponseWriter) writeLen(n int) {
	_, _ = fmt.Fprintf(rw.writer, "*%d\r\n", n)
}

func inline(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s)
}

// scanBulkString reads a RESP bulk string message from a bufio.reader
//
// It also strips it from its prefix and trailing CRLF character, returning
// only the interpretable content of the message.
func scanBulkString(r *bufio.Reader) (string, error) {
	line, err := scanLine(r)
	if err != nil {
		return "", err
	}

	switch Prefix(line[0]) {
	case BulkStringPrefix:
		length, err := strconv.Atoi(line[1 : len(line)-2])
		if err != nil {
			return "", err
		}

		if length < 0 {
			return line, nil
		}

		buf := make([]byte, length+2)
		for pos := 0; pos < length+2; {
			n, err := r.Read(buf[pos:])
			if err != nil {
				return "", err
			}

			pos += n
		}

		return string(buf[:len(buf)-2]), nil
	default:
		return "", ErrInvalidSyntax
	}
}

// scanArray reads a RESP array message from a bufio.Reader.
//
// It strips it from its prefix and trailing CRLF character,
// returning only the interpretable content of the message.
func scanArray(r *bufio.Reader) ([]string, error) {
	line, err := scanLine(r)
	if err != nil {
		return nil, err
	}

	if len(line) < 3 {
		return nil, ErrInvalidSyntax
	}

	if Prefix(line[0]) != ArrayPrefix {
		return nil, ErrInvalidSyntax
	}

	length, err := strconv.Atoi(line[1 : len(line)-2])
	if err != nil {
		return nil, err
	}

	var elements []string
	for ; length > 0; length-- {
		next, err := scanBulkString(r)
		if err != nil {
			return nil, err
		}

		elements = append(elements, next)
	}

	return elements, nil
}

// scanLine reads a RESP protocol line from a bufio.Reader.
func scanLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	if len(line) < 3 {
		return "", ErrInvalidSyntax
	}

	return line, nil
}
