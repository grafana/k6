package dns

import "github.com/miekg/dns"

// RecordType represents a DNS record type.
//
// It is a string type with a restricted set of values.
// The values are the DNS record types supported by the module.
//
// The currently supported values are:
// - A
// - AAAA
//
// The supported values are the ones that are most likely to be
// used by the users of this extension and package, as they are
// those returning IP addresses. Other record types could be
// supported later on, as long as we extend our resolver's logic
// to support them.
//
// We use a custom type to restrict the set of values, and to
// avoid leaking the underlying dns package's types to the
// users of the reusable abstractions defined by this module.
//
//go:generate enumer -type=RecordType -trimprefix RecordType -output record_type_gen.go
type RecordType uint16

// Note that we aligned the values of the RecordType enum values with the
// corresponding values of the dns package's types for convenience.
//
// Note that the RecordType enum values are explicitly typed to allow enumer
// to detect them.
const (
	RecordTypeA    = RecordType(dns.TypeA)
	RecordTypeAAAA = RecordType(dns.TypeAAAA)
	RecordTypeTXT  = RecordType(dns.TypeTXT)
)
