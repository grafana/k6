package dns

import (
	"context"
	"time"
)

// MessageWriter is used by a DNS handler to serve a DNS query.
type MessageWriter interface {
	// Authoritative sets the Authoritative Answer (AA) bit of the header.
	Authoritative(bool)
	// Recursion sets the Recursion Available (RA) bit of the header.
	Recursion(bool)
	// Status sets the Response code (RCODE) bits of the header.
	Status(RCode)

	// Answer adds a record to the answers section.
	Answer(string, time.Duration, Record)
	// Authority adds a record to the authority section.
	Authority(string, time.Duration, Record)
	// Additional adds a record to the additional section
	Additional(string, time.Duration, Record)

	// Recur forwards the request query upstream, and returns the response
	// message or error.
	Recur(context.Context) (*Message, error)

	// Reply sends the response message.
	//
	// For large messages sent over a UDP connection, an ErrTruncatedMessage
	// error is returned if the message was truncated.
	Reply(context.Context) error
}

type messageWriter struct {
	msg *Message
}

func (w *messageWriter) Authoritative(aa bool) { w.msg.Authoritative = aa }
func (w *messageWriter) Recursion(ra bool)     { w.msg.RecursionAvailable = ra }
func (w *messageWriter) Status(rc RCode)       { w.msg.RCode = rc }

func (w *messageWriter) Answer(fqdn string, ttl time.Duration, rec Record) {
	w.msg.Answers = append(w.msg.Answers, w.rr(fqdn, ttl, rec))
}

func (w *messageWriter) Authority(fqdn string, ttl time.Duration, rec Record) {
	w.msg.Authorities = append(w.msg.Authorities, w.rr(fqdn, ttl, rec))
}

func (w *messageWriter) Additional(fqdn string, ttl time.Duration, rec Record) {
	w.msg.Additionals = append(w.msg.Additionals, w.rr(fqdn, ttl, rec))
}

func (w *messageWriter) rr(fqdn string, ttl time.Duration, rec Record) Resource {
	return Resource{
		Name:   fqdn,
		Class:  ClassIN,
		TTL:    ttl,
		Record: rec,
	}
}
