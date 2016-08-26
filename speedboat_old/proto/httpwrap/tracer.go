package httpwrap

import (
	log "github.com/Sirupsen/logrus"
	"net"
	"net/http/httptrace"
	"time"
)

// A Tracer uses Go 1.7's new "net/http/httptrace" package to collect detailed request metrics.
// Cheers, love, the cavalry's here.
type Tracer struct {
	// Duration of the full request.
	Duration time.Duration
	// Time between the start of the request until the first response byte is obtained.
	TimeToFirstByte time.Duration
	// Time between the request is sent and the first byte is obtained.
	TimeWaiting time.Duration

	// Timings for various parts of the request cycle.
	TimeForDNS          time.Duration
	TimeForConnect      time.Duration
	TimeForWriteHeaders time.Duration
	TimeForWriteBody    time.Duration

	// Non-timing related connection info.
	// TODO: Find a way to report this; stats currently only handles float64s.
	ConnAddr     net.Addr
	ConnReused   bool
	ConnWasIdle  bool
	ConnIdleTime time.Duration

	// Reference points.
	startTimeDNS        time.Time
	startTimeConnect    time.Time
	endTimeConnect      time.Time
	endTimeWriteHeaders time.Time
	endTimeWriteBody    time.Time
}

// MakeClientTrace makes a ClientTrace for use with the httptrace package.
func (t *Tracer) MakeClientTrace() httptrace.ClientTrace {
	return httptrace.ClientTrace{
		DNSStart:             t.dnsStart,
		DNSDone:              t.dnsDone,
		ConnectStart:         t.connectStart,
		ConnectDone:          t.connectDone,
		GotConn:              t.gotConn,
		WroteHeaders:         t.wroteHeaders,
		WroteRequest:         t.wroteRequest,
		GotFirstResponseByte: t.gotFirstResponseByte,
	}
}

// RequestDone tells the tracer that the request has been fully completed, and is needed to fully
// compute timings. Should not be needed in the future: https://github.com/golang/go/issues/16400
func (t *Tracer) RequestDone() {
	log.Debug("Request Done")
	if t.startTimeConnect.IsZero() {
		t.Duration = 0
		return
	}
	t.Duration = time.Since(t.startTimeConnect)
}

func (t *Tracer) dnsStart(info httptrace.DNSStartInfo) {
	log.Debug("DNS Start")
	t.startTimeDNS = time.Now()
}

func (t *Tracer) dnsDone(info httptrace.DNSDoneInfo) {
	log.Debug("DNS Done")
	t.TimeForDNS = time.Since(t.startTimeDNS)
}

func (t *Tracer) connectStart(network, addr string) {
	log.Debug("Connect Start")
	// Dual-stack dials will call this multiple times, then discard all but the first successful
	// connection. For our purposes, connection time is the time between the FIRST outgoing
	// connection attempt, to the FIRST successful connection.
	if !t.startTimeConnect.IsZero() {
		log.Debug("-> Duplicate!")
		return
	}
	t.startTimeConnect = time.Now()
	t.TimeForConnect = 0
}

func (t *Tracer) connectDone(network, addr string, err error) {
	log.Debug("Connect Done")
	// Discard all but the first successful connection. See ConnectStart().
	if t.TimeForConnect != 0 {
		log.Debug("-> Duplicate!")
		return
	}
	t.endTimeConnect = time.Now()
	t.TimeForConnect = t.endTimeConnect.Sub(t.startTimeConnect)
}

func (t *Tracer) gotConn(info httptrace.GotConnInfo) {
	log.Debug("Got Conn")
	if info.Reused {
		t.startTimeConnect = time.Now()
		t.endTimeConnect = t.startTimeConnect
		t.TimeForConnect = 0
	}

	t.ConnAddr = info.Conn.RemoteAddr()
	t.ConnReused = info.Reused
	t.ConnWasIdle = info.WasIdle
	t.ConnIdleTime = info.IdleTime
}

func (t *Tracer) wroteHeaders() {
	log.Debug("Wrote Headers")
	t.endTimeWriteHeaders = time.Now()
	t.TimeForWriteHeaders = t.endTimeWriteHeaders.Sub(t.endTimeConnect)
}

func (t *Tracer) wroteRequest(info httptrace.WroteRequestInfo) {
	log.Debug("Wrote Request")
	t.endTimeWriteBody = time.Now()
	t.TimeForWriteBody = t.endTimeWriteBody.Sub(t.endTimeWriteHeaders)
}

func (t *Tracer) gotFirstResponseByte() {
	log.Debug("Got First Response Byte")
	t.TimeToFirstByte = time.Since(t.startTimeConnect)
	t.TimeWaiting = time.Since(t.endTimeWriteBody)
}
