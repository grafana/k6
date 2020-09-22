package dns

import (
	"context"
	"net"
	"sync/atomic"
)

// Client is a DNS client.
type Client struct {
	// Transport manages connections to DNS servers.
	Transport AddrDialer

	// Resolver is a handler that may answer all or portions of a query.
	// Any questions answered by the handler are not sent to the upstream
	// server.
	Resolver Handler

	id uint32
}

// Dial dials a DNS server and returns a net Conn that reads and writes DNS
// messages.
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, err
		}

		conn, err := c.dial(ctx, addr)
		if err != nil {
			return nil, err
		}

		return &streamSession{
			session: session{
				Conn:    conn,
				addr:    addr,
				client:  c,
				msgerrc: make(chan msgerr),
			},
		}, nil
	case "udp", "udp4", "udp6":
		addr, err := net.ResolveUDPAddr(network, address)
		if err != nil {
			return nil, err
		}

		conn, err := c.dial(ctx, addr)
		if err != nil {
			return nil, err
		}

		return &packetSession{
			session: session{
				Conn:    conn,
				addr:    addr,
				client:  c,
				msgerrc: make(chan msgerr),
			},
		}, nil
	default:
		return nil, ErrUnsupportedNetwork
	}
}

// Do sends a DNS query to a server and returns the response message.
func (c *Client) Do(ctx context.Context, query *Query) (*Message, error) {
	conn, err := c.dial(ctx, query.RemoteAddr)
	if err != nil {
		return nil, err
	}

	if t, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(t); err != nil {
			return nil, err
		}
	}

	return c.do(ctx, conn, query)
}

func (c *Client) dial(ctx context.Context, addr net.Addr) (Conn, error) {
	tport := c.Transport
	if tport == nil {
		tport = new(Transport)
	}

	return tport.DialAddr(ctx, addr)
}

func (c *Client) do(ctx context.Context, conn Conn, query *Query) (*Message, error) {
	if c.Resolver == nil {
		return c.roundtrip(conn, query)
	}

	w := &clientWriter{
		messageWriter: &messageWriter{
			msg: response(query.Message),
		},

		req:  request(query.Message),
		addr: query.RemoteAddr,
		conn: conn,

		roundtrip: c.roundtrip,
	}

	c.Resolver.ServeDNS(ctx, w, query)
	if w.err != nil {
		return nil, w.err
	}
	return response(w.msg), nil
}

func (c *Client) roundtrip(conn Conn, query *Query) (*Message, error) {
	id := query.ID

	msg := *query.Message
	msg.ID = c.nextID()

	if err := conn.Send(&msg); err != nil {
		return nil, err
	}

	if err := conn.Recv(&msg); err != nil {
		return nil, err
	}
	msg.ID = id

	return &msg, nil
}

const idMask = (1 << 16) - 1

func (c *Client) nextID() int {
	return int(atomic.AddUint32(&c.id, 1) & idMask)
}

type clientWriter struct {
	*messageWriter

	req *Message
	err error

	addr net.Addr
	conn Conn

	roundtrip func(Conn, *Query) (*Message, error)
}

func (w *clientWriter) Recur(context.Context) (*Message, error) {
	qs := make([]Question, 0, len(w.req.Questions))
	for _, q := range w.req.Questions {
		if !questionMatched(q, w.msg) {
			qs = append(qs, q)
		}
	}
	w.req.Questions = qs

	req := &Query{
		Message:    w.req,
		RemoteAddr: w.addr,
	}

	msg, err := w.roundtrip(w.conn, req)
	if err != nil {
		w.err = err
	}

	return msg, err
}

func (w *clientWriter) Reply(context.Context) error {
	return ErrUnsupportedOp
}

func request(msg *Message) *Message {
	req := new(Message)
	*req = *msg // shallow copy

	return req
}

func questionMatched(q Question, msg *Message) bool {
	mrs := [3][]Resource{
		msg.Answers,
		msg.Authorities,
		msg.Additionals,
	}

	for _, rs := range mrs {
		for _, res := range rs {
			if res.Name == q.Name {
				return true
			}
		}
	}

	return false
}

func writeMessage(w MessageWriter, msg *Message) {
	w.Status(msg.RCode)
	w.Authoritative(msg.Authoritative)
	w.Recursion(msg.RecursionAvailable)

	for _, res := range msg.Answers {
		w.Answer(res.Name, res.TTL, res.Record)
	}
	for _, res := range msg.Authorities {
		w.Authority(res.Name, res.TTL, res.Record)
	}
	for _, res := range msg.Additionals {
		w.Additional(res.Name, res.TTL, res.Record)
	}
}
