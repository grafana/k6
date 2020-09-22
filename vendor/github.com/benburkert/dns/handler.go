package dns

import (
	"context"
	"strings"
)

// Handler responds to a DNS query.
//
// ServeDNS should build the reply message using the MessageWriter, and may
// optionally call the Reply method. Returning signals that the request is
// finished and the response is ready to send.
//
// A recursive handler may call the Recur method of the MessageWriter to send
// an query upstream. Only unanswered questions are included in the upstream
// query.
type Handler interface {
	ServeDNS(context.Context, MessageWriter, *Query)
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as
// DNS handlers. If f is a function with the appropriate signature,
// HandlerFunc(f) is a Handler that calls f.
type HandlerFunc func(context.Context, MessageWriter, *Query)

// ServeDNS calls f(w, r).
func (f HandlerFunc) ServeDNS(ctx context.Context, w MessageWriter, r *Query) {
	f(ctx, w, r)
}

// Recursor forwards a query and copies the response.
func Recursor(ctx context.Context, w MessageWriter, r *Query) {
	msg, err := w.Recur(ctx)
	if err != nil {
		w.Status(ServFail)
		return
	}

	writeMessage(w, msg)
}

// Refuse responds to all queries with a "Query Refused" message.
func Refuse(ctx context.Context, w MessageWriter, r *Query) {
	w.Status(Refused)
}

// ResolveMux is a DNS query multiplexer. It matches a question type and name
// suffix to a Handler.
type ResolveMux struct {
	tbl []muxEntry
}

type muxEntry struct {
	typ    Type
	suffix string
	h      Handler
}

// Handle registers the handler for the given question type and name suffix.
func (m *ResolveMux) Handle(typ Type, suffix string, h Handler) {
	m.tbl = append(m.tbl, muxEntry{typ: typ, suffix: suffix, h: h})
}

// ServeDNS dispatches the query to the handler(s) whose pattern most closely
// matches each question.
func (m *ResolveMux) ServeDNS(ctx context.Context, w MessageWriter, r *Query) {
	var muxw *muxWriter
	for _, q := range r.Questions {
		h := m.lookup(q)

		muxm := new(Message)
		*muxm = *r.Message
		muxm.Questions = []Question{q}

		muxr := new(Query)
		*muxr = *r
		muxr.Message = muxm

		muxw = &muxWriter{
			messageWriter: &messageWriter{
				msg: response(muxr.Message),
			},

			recurc: make(chan msgerr),
			replyc: make(chan msgerr),

			next: muxw,
		}

		go m.serveMux(ctx, h, muxw, muxr)
	}

	if me, ok := <-muxw.recurc; ok {
		writeMessage(w, me.msg)
		msg, err := w.Recur(ctx)
		muxw.recurc <- msgerr{msg, err}
	}

	me := <-muxw.replyc
	writeMessage(w, me.msg)

	if err := w.Reply(ctx); err != nil {
		muxw.replyc <- msgerr{nil, err}
	}
}

var recursiveHandler = HandlerFunc(func(ctx context.Context, w MessageWriter, r *Query) {
	msg, err := w.Recur(ctx)
	if err != nil {
		w.Status(ServFail)
		return
	}

	w.Status(msg.RCode)
	w.Authoritative(msg.Authoritative)
	w.Recursion(msg.RecursionAvailable)

	for _, rec := range msg.Answers {
		w.Answer(rec.Name, rec.TTL, rec.Record)
	}
	for _, rec := range msg.Authorities {
		w.Authority(rec.Name, rec.TTL, rec.Record)
	}
	for _, rec := range msg.Additionals {
		w.Additional(rec.Name, rec.TTL, rec.Record)
	}
})

func (m *ResolveMux) lookup(q Question) Handler {
	for _, e := range m.tbl {
		if e.typ != q.Type && e.typ != TypeANY {
			continue
		}
		if strings.HasSuffix(q.Name, e.suffix) {
			return e.h
		}
	}

	return recursiveHandler
}

func (m *ResolveMux) serveMux(ctx context.Context, h Handler, w *muxWriter, r *Query) {
	h.ServeDNS(ctx, w, r)
	w.finish(ctx)
}

type muxWriter struct {
	*messageWriter

	recurc, replyc chan msgerr

	next *muxWriter
}

func (w muxWriter) Recur(ctx context.Context) (*Message, error) {
	var (
		nextOK bool

		msg = request(w.msg)
	)

	if w.next != nil {
		var me msgerr
		if me, nextOK = <-w.next.recurc; nextOK {
			mergeRequests(msg, me.msg)
		}
	}
	w.recurc <- msgerr{msg, nil}

	me := <-w.recurc
	if nextOK {
		w.next.recurc <- me
	}
	if me.err != nil {
		return nil, me.err
	}
	return responseFor(w.msg.Questions[0], me.msg), nil
}

func (w muxWriter) Reply(ctx context.Context) error {
	msg := response(w.msg)
	if w.next != nil {
		if me, ok := <-w.next.recurc; ok {
			w.recurc <- me
			me = <-w.recurc
			w.next.recurc <- me
		}

		me, ok := <-w.next.replyc
		if !ok || me.err != nil {
			panic("impossible")
		}
		mergeResponses(msg, me.msg)
	}
	close(w.recurc)
	w.replyc <- msgerr{msg, nil}

	me := <-w.replyc
	if w.next != nil {
		w.next.replyc <- me
	}

	close(w.replyc)
	w.replyc = nil

	return me.err
}

func (w muxWriter) finish(ctx context.Context) {
	if w.replyc != nil {
		w.Reply(ctx)
	}
}

func mergeRequests(to, from *Message) {
	if from.OpCode > to.OpCode {
		to.OpCode = from.OpCode
	}
	to.RecursionDesired = to.RecursionDesired || from.RecursionDesired
	to.Questions = append(from.Questions, to.Questions...)
}

func mergeResponses(to, from *Message) {
	to.Authoritative = to.Authoritative && from.Authoritative
	to.RecursionAvailable = to.RecursionAvailable || from.RecursionAvailable
	if from.RCode > to.RCode {
		to.RCode = from.RCode
	}
	to.Questions = append(from.Questions, to.Questions...)
	to.Answers = append(from.Answers, to.Answers...)
	to.Authorities = append(from.Authorities, to.Authorities...)
	to.Additionals = append(from.Additionals, to.Additionals...)
}

func responseFor(q Question, res *Message) *Message {
	msg := response(res)

	var answers []Resource
	for _, a := range res.Answers {
		if a.Name == q.Name {
			answers = append(answers, a)
		}
	}
	msg.Answers = answers

	return msg
}
