package dns

import "errors"

// errUnsupportedRecordType is an error that is returned when a record type is not supported by
// the module.
var errUnsupportedRecordType = errors.New("unsupported record type")

// Error represents a DNS error.
type Error struct {
	// Name holds the descriptive name of the error.
	Name string `json:"name"`

	// Message holds the error message.
	Message string `json:"message"`

	// kind holds the underlying error kind, matching the DNS response codes defined in
	// DNS' [specification] of RCodes.
	//
	// [specification]: https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-6).
	Kind errorKind `json:"kind"`
}

// Ensure our DNSError implements the error interface
var _ error = &Error{}

// newDNSErrorFrom creates a new DNSError from a DNS response code and a message.
func newDNSError(rcode int, message string) *Error {
	kind := errorKind(rcode)

	return &Error{
		Name:    kind.String(),
		Message: message,
		Kind:    kind,
	}
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.Kind.String() + ": " + e.Message
}

// errorKind represents a DNS error kind, based on DNS Response Codes.
//
//go:generate enumer -type=errorKind -output errors_gen.go
type errorKind int

// DNS error kinds based on DNS Response Codes, see https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml
const (
	// FormatError is a DNS error kind that represents a Format Error.
	// It is based on the DNS Response Code FormErr, as per [RFC1035].
	//
	// [RFC1035]: https://www.iana.org/go/rfc1035
	FormatError errorKind = 1

	// ServerFailure is a DNS error kind that represents a Server Failure.
	// It is based on the DNS Response Code ServFail, as per [RFC1035].
	//
	// [RFC1035]: https://www.iana.org/go/rfc1035
	ServerFailure errorKind = 2

	// NonExistingDomain is a DNS error kind that represents a Non-Existent Domain.
	// It is based on the DNS Response Code NXDomain, as per [RFC1035].
	//
	// [RFC1035]: https://www.iana.org/go/rfc1035
	NonExistingDomain errorKind = 3

	// NotImplemented is a DNS error kind that represents a Not Implemented error.
	// It is based on the DNS Response Code NotImp, as per [RFC1035].
	//
	// [RFC1035]: https://www.iana.org/go/rfc1035
	NotImplemented errorKind = 4

	// Refused is a DNS error kind that represents a Query Refused error.
	// It is based on the DNS Response Code Refused, as per [RFC1035].
	//
	// [RFC1035]: https://www.iana.org/go/rfc1035
	Refused errorKind = 5

	// YXDomain is a DNS error kind that represents a Name Exists when it should not.
	// It is based on the DNS Response Code YXDomain, as per [RFC2136].
	//
	// [RFC2136]: https://www.iana.org/go/rfc2136
	YXDomain errorKind = 6

	// YXRrset is a DNS error kind that represents a RR Set Exists when it should not.
	// It is based on the DNS Response Code YXRRSet, as per [RFC2136].
	//
	// [RFC2136]: https://www.iana.org/go/rfc2136
	YXRrset errorKind = 7

	// NXRrset is a DNS error kind that represents a RR Set that should exist does not.
	// It is based on the DNS Response Code NXRRSet, as per [RFC2136].
	//
	// [RFC2136]: https://www.iana.org/go/rfc2136
	NXRrset errorKind = 8

	// NotAuth is a DNS error kind that represents a Server Not Authoritative for zone.
	// It is based on the DNS Response Code NotAuth, as per [RFC2136].
	//
	// [RFC2136]: https://www.iana.org/go/rfc2136
	NotAuth errorKind = 9

	// NotZone is a DNS error kind that represents a Name not contained in zone.
	// It is based on the DNS Response Code NotZone, as per [RFC2136].
	//
	// [RFC2136]: https://www.iana.org/go/rfc2136
	NotZone errorKind = 10

	// BadVers is a DNS error kind that represents a Bad OPT Version.
	// It is based on the DNS Response Code BadVers, as per [RFC6891].
	//
	// [RFC6891]: https://www.iana.org/go/rfc6891
	BadVers errorKind = 16

	// BadSig is a DNS error kind that represents a TSIG Signature Failure.
	// It is based on the DNS Response Code BadSig, as per [RFC8945].
	//
	// [RFC8945]: https://www.iana.org/go/rfc8945
	BadSig errorKind = 16

	// BadKey is a DNS error kind that represents a Key not recognized.
	// It is based on the DNS Response Code BadKey, as per [RFC8945].
	//
	// [RFC8945]: https://www.iana.org/go/rfc8945
	BadKey errorKind = 17

	// BadTime is a DNS error kind that represents a Signature out of time window.
	// It is based on the DNS Response Code BadTime, as per [RFC8945].
	//
	// [RFC8945]: https://www.iana.org/go/rfc8945
	BadTime errorKind = 18

	// BadMode is a DNS error kind that represents a Bad TKEY Mode.
	// It is based on the DNS Response Code BadMode, as per [RFC2930].
	//
	// [RFC2930]: https://www.iana.org/go/rfc2930
	BadMode errorKind = 19

	// BadName is a DNS error kind that represents a Duplicate key name.
	// It is based on the DNS Response Code BadName, as per [RFC2930].
	//
	// [RFC2930]: https://www.iana.org/go/rfc2930
	BadName errorKind = 20

	// BadAlg is a DNS error kind that represents an Algorithm not supported.
	// It is based on the DNS Response Code BadAlg, as per [RFC2930].
	//
	// [RFC2930]: https://www.iana.org/go/rfc2930
	BadAlg errorKind = 21

	// BadTrunc is a DNS error kind that represents a Bad Truncation.
	// It is based on the DNS Response Code BadTrunc, as per [RFC8945].
	//
	// [RFC8945]: https://www.iana.org/go/rfc8945
	BadTrunc errorKind = 22

	// BadCookie is a DNS error kind that represents a Bad/missing Server Cookie.
	// It is based on the DNS Response Code BadCookie, as per [RFC7873].
	//
	// [RFC7873]: https://www.iana.org/go/rfc7873
	BadCookie errorKind = 23
)
