// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"encoding/binary"
	"errors"
	"net"
	"time"

	"github.com/benburkert/dns/edns"
)

var nbo = binary.BigEndian

// A Type is a type of DNS request and response.
type Type uint16

// A Class is a type of network.
type Class uint16

// An OpCode is a DNS operation code.
type OpCode uint16

// An RCode is a DNS response status code.
type RCode uint16

// Domain Name System (DNS) Parameters.
//
// Taken from https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml
const (
	// Resource Record (RR) TYPEs
	TypeA     Type = 1   // [RFC1035] a host address
	TypeNS    Type = 2   // [RFC1035] an authoritative name server
	TypeCNAME Type = 5   // [RFC1035] the canonical name for an alias
	TypeSOA   Type = 6   // [RFC1035] marks the start of a zone of authority
	TypeWKS   Type = 11  // [RFC1035] a well known service description
	TypePTR   Type = 12  // [RFC1035] a domain name pointer
	TypeHINFO Type = 13  // [RFC1035] host information
	TypeMINFO Type = 14  // [RFC1035] mailbox or mail list information
	TypeMX    Type = 15  // [RFC1035] mail exchange
	TypeTXT   Type = 16  // [RFC1035] text strings
	TypeAAAA  Type = 28  // [RFC3596] IP6 Address
	TypeSRV   Type = 33  // [RFC2782] Server Selection
	TypeDNAME Type = 39  // [RFC6672] DNAME
	TypeOPT   Type = 41  // [RFC6891][RFC3225] OPT
	TypeAXFR  Type = 252 // [RFC1035][RFC5936] transfer of an entire zone
	TypeALL   Type = 255 // [RFC1035][RFC6895] A request for all records the server/cache has available
	TypeCAA   Type = 257 // [RFC6844] Certification Authority Restriction

	TypeANY Type = 0

	// DNS CLASSes
	ClassIN  Class = 1   // [RFC1035] Internet (IN)
	ClassCH  Class = 3   // [] Chaos (CH)
	ClassHS  Class = 4   // [] Hesiod (HS)
	ClassANY Class = 255 // [RFC1035] QCLASS * (ANY)

	// DNS RCODEs
	NoError  RCode = 0 // [RFC1035] No Error
	FormErr  RCode = 1 // [RFC1035] Format Error
	ServFail RCode = 2 // [RFC1035] Server Failure
	NXDomain RCode = 3 // [RFC1035] Non-Existent Domain
	NotImp   RCode = 4 // [RFC1035] Not Implemented
	Refused  RCode = 5 // [RFC1035] Query Refused

	maxPacketLen = 512
)

// NewRecordByType returns a new instance of a Record for a Type.
var NewRecordByType = map[Type]func() Record{
	TypeA:     func() Record { return new(A) },
	TypeNS:    func() Record { return new(NS) },
	TypeCNAME: func() Record { return new(CNAME) },
	TypeSOA:   func() Record { return new(SOA) },
	TypePTR:   func() Record { return new(PTR) },
	TypeMX:    func() Record { return new(MX) },
	TypeTXT:   func() Record { return new(TXT) },
	TypeAAAA:  func() Record { return new(AAAA) },
	TypeSRV:   func() Record { return new(SRV) },
	TypeDNAME: func() Record { return new(DNAME) },
	TypeOPT:   func() Record { return new(OPT) },
	TypeCAA:   func() Record { return new(CAA) },
}

var (
	// ErrNotStarted indicates that the prerequisite information isn't
	// available yet because the previous records haven't been appropriately
	// parsed or skipped.
	ErrNotStarted = errors.New("parsing of this type isn't available yet")

	// ErrSectionDone indicated that all records in the section have been
	// parsed.
	ErrSectionDone = errors.New("parsing of this section has completed")

	errBaseLen            = errors.New("insufficient data for base length type")
	errCalcLen            = errors.New("insufficient data for calculated length type")
	errReserved           = errors.New("segment prefix is reserved")
	errPtrCycle           = errors.New("pointer cycle")
	errInvalidFQDN        = errors.New("invalid FQDN")
	errInvalidPtr         = errors.New("invalid pointer")
	errResourceLen        = errors.New("insufficient data for resource body length")
	errSegTooLong         = errors.New("segment length too long")
	errZeroSegLen         = errors.New("zero length segment")
	errResTooLong         = errors.New("resource length too long")
	errTooManyQuestions   = errors.New("too many Questions to pack (>65535)")
	errTooManyAnswers     = errors.New("too many Answers to pack (>65535)")
	errTooManyAuthorities = errors.New("too many Authorities to pack (>65535)")
	errTooManyAdditionals = errors.New("too many Additionals to pack (>65535)")
	errFieldOverflow      = errors.New("value too large for packed field")
	errUnknownType        = errors.New("unknown resource type")
)

// Message is a DNS message.
type Message struct {
	ID                 int
	Response           bool
	OpCode             OpCode
	Authoritative      bool
	Truncated          bool
	RecursionDesired   bool
	RecursionAvailable bool
	RCode              RCode

	Questions   []Question
	Answers     []Resource
	Authorities []Resource
	Additionals []Resource
}

// Pack encodes m as a byte slice. If b is not nil, m is appended into b.
// Domain name compression is enabled by setting compress.
func (m *Message) Pack(b []byte, compress bool) ([]byte, error) {
	if b == nil {
		b = make([]byte, 0, maxPacketLen)
	}

	var com Compressor
	if compress {
		com = compressor{tbl: make(map[string]int), offset: len(b)}
	}

	var err error
	if b, err = m.packHeader(b); err != nil {
		return nil, err
	}

	for _, q := range m.Questions {
		if b, err = q.Pack(b, com); err != nil {
			return nil, err
		}
	}

	for _, rs := range [3][]Resource{m.Answers, m.Authorities, m.Additionals} {
		for _, r := range rs {
			if b, err = r.Pack(b, com); err != nil {
				return nil, err
			}
		}
	}

	return b, nil
}

// Unpack decodes m from b. Unused bytes are returned.
func (m *Message) Unpack(b []byte) ([]byte, error) {
	dec := decompressor(b)

	var err error
	if b, err = m.unpackHeader(b); err != nil {
		return nil, err
	}

	for i := 0; i < cap(m.Questions); i++ {
		var q Question
		if b, err = q.Unpack(b, dec); err != nil {
			return nil, err
		}
		m.Questions = append(m.Questions, q)
	}
	for i := 0; i < cap(m.Answers); i++ {
		var r Resource
		if b, err = r.Unpack(b, dec); err != nil {
			return nil, err
		}
		m.Answers = append(m.Answers, r)
	}
	for i := 0; i < cap(m.Authorities); i++ {
		var r Resource
		if b, err = r.Unpack(b, dec); err != nil {
			return nil, err
		}
		m.Authorities = append(m.Authorities, r)
	}
	for i := 0; i < cap(m.Additionals); i++ {
		var r Resource
		if b, err = r.Unpack(b, dec); err != nil {
			return nil, err
		}
		m.Additionals = append(m.Additionals, r)
	}

	return b, nil
}

const (
	headerBitQR = 1 << 15 // query/response (response=1)
	headerBitAA = 1 << 10 // authoritative
	headerBitTC = 1 << 9  // truncated
	headerBitRD = 1 << 8  // recursion desired
	headerBitRA = 1 << 7  // recursion available
)

func (m *Message) packHeader(b []byte) ([]byte, error) {
	id := uint16(m.ID)
	if int(id) != m.ID {
		return nil, errFieldOverflow
	}

	opcode := m.OpCode & 0x0F
	if opcode != m.OpCode {
		return nil, errFieldOverflow
	}

	rcode := m.RCode & 0x0F
	if rcode != m.RCode {
		return nil, errFieldOverflow
	}

	bits := uint16(opcode)<<11 | uint16(rcode)
	if m.Response {
		bits |= headerBitQR
	}
	if m.RecursionAvailable {
		bits |= headerBitRA
	}
	if m.RecursionDesired {
		bits |= headerBitRD
	}
	if m.Truncated {
		bits |= headerBitTC
	}
	if m.Authoritative {
		bits |= headerBitAA
	}

	qdcount := uint16(len(m.Questions))
	if int(qdcount) != len(m.Questions) {
		return nil, errTooManyQuestions
	}

	ancount := uint16(len(m.Answers))
	if int(ancount) != len(m.Answers) {
		return nil, errTooManyAnswers
	}

	nscount := uint16(len(m.Authorities))
	if int(nscount) != len(m.Authorities) {
		return nil, errTooManyAuthorities
	}

	arcount := uint16(len(m.Additionals))
	if int(nscount) != len(m.Authorities) {
		return nil, errTooManyAuthorities
	}

	buf := [12]byte{}
	nbo.PutUint16(buf[0:2], id)
	nbo.PutUint16(buf[2:4], bits)
	nbo.PutUint16(buf[4:6], qdcount)
	nbo.PutUint16(buf[6:8], ancount)
	nbo.PutUint16(buf[8:10], nscount)
	nbo.PutUint16(buf[10:12], arcount)
	return append(b, buf[:]...), nil
}

func (m *Message) unpackHeader(b []byte) ([]byte, error) {
	if len(b) < 12 {
		return nil, errResourceLen
	}

	var (
		id      = int(nbo.Uint16(b))
		bits    = nbo.Uint16(b[2:])
		qdcount = nbo.Uint16(b[4:])
		ancount = nbo.Uint16(b[6:])
		nscount = nbo.Uint16(b[8:])
		arcount = nbo.Uint16(b[10:])
	)

	*m = Message{
		ID:                 id,
		Response:           (bits & headerBitQR) > 0,
		OpCode:             OpCode(bits>>11) & 0xF,
		Authoritative:      (bits & headerBitAA) > 0,
		Truncated:          (bits & headerBitTC) > 0,
		RecursionDesired:   (bits & headerBitRD) > 0,
		RecursionAvailable: (bits & headerBitRA) > 0,
		RCode:              RCode(bits) & 0xF,
	}

	if qdcount > 0 {
		m.Questions = make([]Question, 0, qdcount)
	}
	if ancount > 0 {
		m.Answers = make([]Resource, 0, ancount)
	}
	if nscount > 0 {
		m.Authorities = make([]Resource, 0, nscount)
	}
	if arcount > 0 {
		m.Additionals = make([]Resource, 0, arcount)
	}

	return b[12:], nil
}

// A Question is a DNS query.
type Question struct {
	Name  string
	Type  Type
	Class Class
}

// Pack encodes q as a byte slice. If b is not nil, m is appended into b.
func (q Question) Pack(b []byte, com Compressor) ([]byte, error) {
	if com == nil {
		com = compressor{}
	}

	var err error
	if b, err = com.Pack(b, q.Name); err != nil {
		return nil, err
	}

	buf := [4]byte{}
	nbo.PutUint16(buf[:2], uint16(q.Type))
	nbo.PutUint16(buf[2:4], uint16(q.Class))
	return append(b, buf[:]...), nil
}

// Unpack decodes q from b.
func (q *Question) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	if dec == nil {
		dec = decompressor(nil)
	}

	var err error
	if q.Name, b, err = dec.Unpack(b); err != nil {
		return nil, err
	}

	if len(b) < 4 {
		return nil, errResourceLen
	}

	q.Type = Type(nbo.Uint16(b[:2]))
	q.Class = Class(nbo.Uint16(b[2:4]))

	return b[4:], nil
}

// Resource is a DNS resource record (RR).
type Resource struct {
	Name  string
	Class Class
	TTL   time.Duration

	Record
}

// Pack encodes r onto b.
func (r Resource) Pack(b []byte, com Compressor) ([]byte, error) {
	if com == nil {
		com = compressor{}
	}

	var err error
	if b, err = com.Pack(b, r.Name); err != nil {
		return nil, err
	}

	rtype := r.Record.Type()

	ttl := uint32(r.TTL / time.Second)
	if time.Duration(ttl) != r.TTL/time.Second {
		return nil, errFieldOverflow
	}

	rlen, err := r.Record.Length(com)
	if err != nil {
		return nil, err
	}

	rdatalen := uint16(rlen)
	if int(rdatalen) != rlen {
		return nil, errFieldOverflow
	}

	buf := [10]byte{}
	nbo.PutUint16(buf[:2], uint16(rtype))
	nbo.PutUint16(buf[2:4], uint16(r.Class))
	nbo.PutUint32(buf[4:8], ttl)
	nbo.PutUint16(buf[8:10], rdatalen)
	b = append(b, buf[:]...)

	return r.Record.Pack(b, com)
}

// Unpack decodes r from b.
func (r *Resource) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	if r.Name, b, err = dec.Unpack(b); err != nil {
		return nil, err
	}

	if len(b) < 10 {
		return nil, errResourceLen
	}

	rtype := Type(nbo.Uint16(b[:2]))
	r.Class = Class(nbo.Uint16(b[2:4]))
	r.TTL = time.Duration(nbo.Uint32(b[4:8])) * time.Second

	rdlen, b := int(nbo.Uint16(b[8:10])), b[10:]
	if len(b) < rdlen {
		return nil, errResourceLen
	}

	newfn, ok := NewRecordByType[rtype]
	if !ok {
		return nil, errUnknownType
	}

	record := newfn()
	buf, err := record.Unpack(b[:rdlen], dec)
	if err != nil {
		return nil, err
	}
	if len(buf) > 0 {
		return nil, errResTooLong
	}
	r.Record = record

	return b[rdlen:], nil
}

// Record is a DNS record.
type Record interface {
	Type() Type
	Length(Compressor) (int, error)
	Pack([]byte, Compressor) ([]byte, error)
	Unpack([]byte, Decompressor) ([]byte, error)
}

// A A is a DNS A record.
type A struct {
	A net.IP
}

// Type returns the RR type identifier.
func (A) Type() Type { return TypeA }

// Length returns the encoded RDATA size.
func (A) Length(Compressor) (int, error) { return 4, nil }

// Pack encodes a as RDATA.
func (a A) Pack(b []byte, _ Compressor) ([]byte, error) {
	if len(a.A) < 4 {
		return nil, errResourceLen
	}
	return append(b, a.A.To4()...), nil
}

// Unpack decodes a from RDATA in b.
func (a *A) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	if len(b) < 4 {
		return nil, errResourceLen
	}
	if len(a.A) != 4 {
		a.A = make([]byte, 4)
	}
	copy(a.A, b[:4])

	return b[4:], nil
}

// AAAA is a DNS AAAA record.
type AAAA struct {
	AAAA net.IP
}

// Type returns the RR type identifier.
func (AAAA) Type() Type { return TypeAAAA }

// Length returns the encoded RDATA size.
func (AAAA) Length(Compressor) (int, error) { return 16, nil }

// Pack encodes a as RDATA.
func (a AAAA) Pack(b []byte, _ Compressor) ([]byte, error) {
	if len(a.AAAA) != 16 {
		return nil, errResourceLen
	}
	return append(b, a.AAAA...), nil
}

// Unpack decodes a from RDATA in b.
func (a *AAAA) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	if len(b) < 16 {
		return nil, errResourceLen
	}
	if len(a.AAAA) != 16 {
		a.AAAA = make([]byte, 16)
	}
	copy(a.AAAA, b[:16])

	return b[16:], nil
}

// CNAME is a DNS CNAME record.
type CNAME struct {
	CNAME string
}

// Type returns the RR type identifier.
func (CNAME) Type() Type { return TypeCNAME }

// Length returns the encoded RDATA size.
func (c CNAME) Length(com Compressor) (int, error) {
	return com.Length(c.CNAME)
}

// Pack encodes c as RDATA.
func (c CNAME) Pack(b []byte, com Compressor) ([]byte, error) {
	return com.Pack(b, c.CNAME)
}

// Unpack decodes c from RDATA in b.
func (c *CNAME) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	c.CNAME, b, err = dec.Unpack(b)
	return b, err
}

// SOA is a DNS SOA record.
type SOA struct {
	NS      string
	MBox    string
	Serial  int
	Refresh time.Duration
	Retry   time.Duration
	Expire  time.Duration
	MinTTL  time.Duration
}

// Type returns the RR type identifier.
func (SOA) Type() Type { return TypeSOA }

// Length returns the encoded RDATA size.
func (s SOA) Length(com Compressor) (int, error) {
	n, err := com.Length(s.NS, s.MBox)
	if err != nil {
		return 0, err
	}
	return n + 20, nil
}

// Pack encodes s as RDATA.
func (s SOA) Pack(b []byte, com Compressor) ([]byte, error) {
	var err error
	if b, err = com.Pack(b, s.NS); err != nil {
		return nil, err
	}
	if b, err = com.Pack(b, s.MBox); err != nil {
		return nil, err
	}

	var (
		serial  = uint32(s.Serial)
		refresh = int32(s.Refresh / time.Second)
		retry   = int32(s.Retry / time.Second)
		expire  = int32(s.Expire / time.Second)
		minimum = uint32(s.MinTTL / time.Second)
	)

	if int(serial) != s.Serial {
		return nil, errFieldOverflow
	}
	if time.Duration(refresh) != s.Refresh/time.Second {
		return nil, errFieldOverflow
	}
	if time.Duration(retry) != s.Retry/time.Second {
		return nil, errFieldOverflow
	}
	if time.Duration(expire) != s.Expire/time.Second {
		return nil, errFieldOverflow
	}
	if time.Duration(minimum) != s.MinTTL/time.Second {
		return nil, errFieldOverflow
	}

	buf := [20]byte{}
	nbo.PutUint32(buf[:4], serial)
	nbo.PutUint32(buf[4:8], uint32(refresh))
	nbo.PutUint32(buf[8:12], uint32(retry))
	nbo.PutUint32(buf[12:16], uint32(expire))
	nbo.PutUint32(buf[16:], minimum)

	return append(b, buf[:]...), nil
}

// Unpack decodes s from RDATA in b.
func (s *SOA) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	if s.NS, b, err = dec.Unpack(b); err != nil {
		return nil, err
	}
	if s.MBox, b, err = dec.Unpack(b); err != nil {
		return nil, err
	}

	if len(b) < 20 {
		return nil, errResourceLen
	}

	var (
		serial  = nbo.Uint32(b[:4])
		refresh = int32(nbo.Uint32(b[4:8]))
		retry   = int32(nbo.Uint32(b[8:12]))
		expire  = int32(nbo.Uint32(b[12:16]))
		minimum = nbo.Uint32(b[16:20])
	)

	s.Serial = int(serial)
	s.Refresh = time.Duration(refresh) * time.Second
	s.Retry = time.Duration(retry) * time.Second
	s.Expire = time.Duration(expire) * time.Second
	s.MinTTL = time.Duration(minimum) * time.Second

	return b[20:], nil
}

// PTR is a DNS PTR record.
type PTR struct {
	PTR string
}

// Type returns the RR type identifier.
func (PTR) Type() Type { return TypePTR }

// Length returns the encoded RDATA size.
func (p PTR) Length(com Compressor) (int, error) {
	return com.Length(p.PTR)
}

// Pack encodes p as RDATA.
func (p PTR) Pack(b []byte, com Compressor) ([]byte, error) {
	return com.Pack(b, p.PTR)
}

// Unpack decodes p from RDATA in b.
func (p *PTR) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	p.PTR, b, err = dec.Unpack(b)
	return b, err
}

// MX is a DNS MX record.
type MX struct {
	Pref int
	MX   string
}

// Type returns the RR type identifier.
func (MX) Type() Type { return TypeMX }

// Length returns the encoded RDATA size.
func (m MX) Length(com Compressor) (int, error) {
	n, err := com.Length(m.MX)
	if err != nil {
		return 0, err
	}
	return n + 2, nil
}

// Pack encodes m as RDATA.
func (m MX) Pack(b []byte, com Compressor) ([]byte, error) {
	pref := uint16(m.Pref)
	if int(pref) != m.Pref {
		return nil, errFieldOverflow
	}

	buf := [2]byte{}
	nbo.PutUint16(buf[:], pref)

	return com.Pack(append(b, buf[:]...), m.MX)
}

// Unpack decodes m from RDATA in b.
func (m *MX) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	if len(b) < 2 {
		return nil, errResourceLen
	}

	m.Pref = int(nbo.Uint16(b[:2]))

	var err error
	m.MX, b, err = dec.Unpack(b[2:])
	return b, err
}

// NS is a DNS MX record.
type NS struct {
	NS string
}

// Type returns the RR type identifier.
func (NS) Type() Type { return TypeNS }

// Length returns the encoded RDATA size.
func (n NS) Length(com Compressor) (int, error) {
	return com.Length(n.NS)
}

// Pack encodes n as RDATA.
func (n NS) Pack(b []byte, com Compressor) ([]byte, error) {
	return com.Pack(b, n.NS)
}

// Unpack decodes n from RDATA in b.
func (n *NS) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	n.NS, b, err = dec.Unpack(b)
	return b, err
}

// TXT is a DNS TXT record.
type TXT struct {
	TXT []string
}

// Type returns the RR type identifier.
func (TXT) Type() Type { return TypeTXT }

// Length returns the encoded RDATA size.
func (t TXT) Length(_ Compressor) (int, error) {
	var n int
	for _, s := range t.TXT {
		n += 1 + len(s)
	}
	return n, nil
}

// Pack encodes t as RDATA.
func (t TXT) Pack(b []byte, _ Compressor) ([]byte, error) {
	for _, s := range t.TXT {
		if len(s) > 255 {
			return nil, errSegTooLong
		}

		b = append(append(b, byte(len(s))), []byte(s)...)
	}
	return b, nil
}

// Unpack decodes t from RDATA in b.
func (t *TXT) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	var txts []string
	for len(b) > 0 {
		txtlen := int(b[0])
		if len(b) < txtlen+1 {
			return nil, errResourceLen
		}

		txts = append(txts, string(b[1:1+txtlen]))
		b = b[1+txtlen:]
	}

	t.TXT = txts
	return nil, nil
}

// SRV is a DNS SRV record.
type SRV struct {
	Priority int
	Weight   int
	Port     int
	Target   string // Not compressed as per RFC 2782.
}

// Type returns the RR type identifier.
func (SRV) Type() Type { return TypeSRV }

// Length returns the encoded RDATA size.
func (s SRV) Length(_ Compressor) (int, error) {
	n, err := compressor{}.Length(s.Target)
	if err != nil {
		return 0, err
	}
	return n + 6, nil
}

// Pack encodes s as RDATA.
func (s SRV) Pack(b []byte, _ Compressor) ([]byte, error) {
	var (
		priority = uint16(s.Priority)
		weight   = uint16(s.Weight)
		port     = uint16(s.Port)
	)

	if int(priority) != s.Priority {
		return nil, errFieldOverflow
	}
	if int(weight) != s.Weight {
		return nil, errFieldOverflow
	}
	if int(port) != s.Port {
		return nil, errFieldOverflow
	}

	buf := [6]byte{}
	nbo.PutUint16(buf[:2], priority)
	nbo.PutUint16(buf[2:4], weight)
	nbo.PutUint16(buf[4:], port)

	return compressor{}.Pack(append(b, buf[:]...), s.Target)
}

// Unpack decodes s from RDATA in b.
func (s *SRV) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	if len(b) < 6 {
		return nil, errResourceLen
	}

	s.Priority = int(nbo.Uint16(b[:2]))
	s.Weight = int(nbo.Uint16(b[2:4]))
	s.Port = int(nbo.Uint16(b[4:6]))

	var err error
	s.Target, b, err = decompressor(nil).Unpack(b[6:])
	return b, err
}

// DNAME is a DNS DNAME record.
type DNAME struct {
	DNAME string
}

// Type returns the RR type identifier.
func (DNAME) Type() Type { return TypeDNAME }

// Length returns the encoded RDATA size.
func (d DNAME) Length(com Compressor) (int, error) {
	return com.Length(d.DNAME)
}

// Pack encodes c as RDATA.
func (d DNAME) Pack(b []byte, com Compressor) ([]byte, error) {
	return com.Pack(b, d.DNAME)
}

// Unpack decodes c from RDATA in b.
func (d *DNAME) Unpack(b []byte, dec Decompressor) ([]byte, error) {
	var err error
	d.DNAME, b, err = dec.Unpack(b)
	return b, err
}

// OPT is a DNS OPT record.
type OPT struct {
	Options []edns.Option
}

// Type returns the RR type identifier.
func (o OPT) Type() Type { return TypeOPT }

// Length returns the encoded RDATA size.
func (o OPT) Length(_ Compressor) (int, error) {
	var n int
	for _, opt := range o.Options {
		n += opt.Length()
	}
	return n, nil
}

// Pack encodes o as RDATA.
func (o OPT) Pack(b []byte, _ Compressor) ([]byte, error) {
	var err error
	for _, opt := range o.Options {
		if b, err = opt.Pack(b); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Unpack decodes o from RDATA in b.
func (o *OPT) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	o.Options = nil

	var err error
	for len(b) > 0 {
		var opt edns.Option
		if b, err = opt.Unpack(b); err != nil {
			return nil, err
		}
		o.Options = append(o.Options, opt)
	}
	return b, nil
}

// type CAA is a DNS CAA record.
type CAA struct {
	IssuerCritical bool

	Tag   string
	Value string
}

// Type returns the RR type identifier.
func (CAA) Type() Type { return TypeCAA }

// Length returns the encoded RDATA size.
func (c CAA) Length(_ Compressor) (int, error) {
	return 2 + len(c.Tag) + len(c.Value), nil
}

// Pack encodes c as RDATA.
func (c CAA) Pack(b []byte, _ Compressor) ([]byte, error) {
	buf := make([]byte, 2, 2+len(c.Tag)+len(c.Value))

	if c.IssuerCritical {
		buf[0] = 1
	}

	tagLength := len(c.Tag)
	if tagLength == 0 {
		return nil, errZeroSegLen
	}
	if tagLength > 255 {
		return nil, errSegTooLong
	}
	buf[1] = byte(tagLength)

	buf = append(buf, []byte(c.Tag)...)
	buf = append(buf, []byte(c.Value)...)

	return append(b, buf...), nil
}

// Unpack decodes c from RDATA in b.
func (c *CAA) Unpack(b []byte, _ Decompressor) ([]byte, error) {
	if len(b) < 2 {
		return nil, errResourceLen
	}

	if b[0]&0x01 > 0 {
		c.IssuerCritical = true
	}

	tagLength := int(b[1])
	if tagLength == 0 {
		return nil, errZeroSegLen
	}
	if 2+tagLength > len(b) {
		return nil, errResourceLen
	}

	c.Tag = string(b[2 : 2+tagLength])
	c.Value = string(b[2+tagLength:])

	return nil, nil
}
