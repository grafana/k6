// Package edns provides EDNS0 (RFC6891) support.
package edns

import (
	"encoding/binary"
	"errors"
	"io"
)

var nbo = binary.BigEndian

// An OptionCode is a EDNS0 option code.
type OptionCode uint16

// DNS EDNS0 Option Codes (OPT).
//
// Taken from https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-11
const (
	// 0	Reserved		[RFC6891]
	OptionCodeLLQ  OptionCode = 1 // On-hold  [RFC6891]
	OptionCodeUL   OptionCode = 2 // On-hold  [http://files.dns-sd.org/draft-sekar-dns-llq.txt]
	OptionCodeNSID OptionCode = 3 // Standard [http://files.dns-sd.org/draft-sekar-dns-ul.txt]
	// 4	Reserved		[draft-cheshire-edns0-owner-option]
	OptionCodeDAU              OptionCode = 5  // Standard [RFC6975]
	OptionCodeDHU              OptionCode = 6  // Standard [RFC6975]
	OptionCodeN3U              OptionCode = 7  // Standard [RFC6975]
	OptionCodeEDNSClientSubnet OptionCode = 8  // Optional [RFC7871]
	OptionCodeEDNSExpire       OptionCode = 9  // Optional [RFC7314]
	OptionCodeCookie           OptionCode = 10 // Standard [RFC7873]
	OptionCodeEDNSTCPKeepAlive OptionCode = 11 // Standard [RFC7828]
	OptionCodePadding          OptionCode = 12 // Standard [RFC7830]
	OptionCodeChain            OptionCode = 13 // Standard [RFC7901]
	OptionCodeEDNSKeyTag       OptionCode = 14 // Optional [RFC8145]
	// 15-26945	Unassigned
	OptionCodeDeviceID OptionCode = 26946 // Optional [https://docs.umbrella.com/developer/networkdevices-api/identifying-dns-traffic2][Brian_Hartvigsen]
	// 26947-65000	Unassigned
	// 65001-65534	Reserved for Local/Experimental Use	[RFC6891]
	// 65535	Reserved for future expansion		[RFC6891]
)

var errOptionLen = errors.New("insufficient data for option length")

// Option is a EDNS0 option.
type Option struct {
	Code OptionCode
	Data []byte
}

// Length returns the encoded RDATA size.
func (o Option) Length() int { return 4 + len(o.Data) }

// Pack encodes o as RDATA.
func (o Option) Pack(b []byte) ([]byte, error) {
	var (
		code   = uint16(o.Code)
		length = uint16(len(o.Data))
	)

	buf := make([]byte, o.Length())
	nbo.PutUint16(buf[:2], code)
	nbo.PutUint16(buf[2:4], length)
	copy(buf[4:], o.Data)

	return append(b, buf[:]...), nil
}

// Unpack decodes o from RDATA in b.
func (o *Option) Unpack(b []byte) ([]byte, error) {
	if len(b) < 4 {
		return nil, errOptionLen
	}

	o.Code = OptionCode(nbo.Uint16(b[:2]))
	l := int(nbo.Uint16(b[2:4]))

	if len(b) < 4+l {
		return nil, io.ErrShortBuffer
	}

	o.Data = make([]byte, l)
	copy(o.Data, b[4:])

	return b[4+l:], nil
}
