package dnsr

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// RR represents a DNS resource record.
type RR struct {
	Name   string
	Type   string
	Value  string
	TTL    time.Duration
	Expiry time.Time
}

// RRs represents a slice of DNS resource records.
type RRs []RR

// emptyRRs is an empty, non-nil slice of RRs.
// It is used to save allocations at runtime.
var emptyRRs = RRs{}

// ICANN specifies that DNS servers should return the special value 127.0.53.53
// for A record queries of TLDs that have recently entered the root zone,
// that have a high likelyhood of colliding with private DNS names.
// The record returned is a notice to network administrators to adjust their
// DNS configuration.
// https://www.icann.org/resources/pages/name-collision-2013-12-06-en#127.0.53.53
const NameCollision = "127.0.53.53"

// String returns a string representation of an RR in zone-file format.
func (rr *RR) String() string {
	if rr.Expiry.IsZero() {
		return rr.Name + "\t      3600\tIN\t" + rr.Type + "\t" + rr.Value
	} else {
		ttl := ttlString(rr.TTL)
		return rr.Name + "\t" + ttl + "\t" + rr.Type + "\t" + rr.Value
	}
}

// ttlString constructs the TTL field of an RR string.
func ttlString(ttl time.Duration) string {
	seconds := int(ttl.Seconds())
	return fmt.Sprintf("%10d", seconds)
}

// convertRR converts a dns.RR to an RR.
// If the RR is not a type that this package uses,
// It will attempt to translate this if there are enough parameters
// Should all translation fail, it returns an undefined RR and false.
func convertRR(drr dns.RR, expire bool) (RR, bool) {
	var ttl time.Duration
	var expiry time.Time
	if expire {
		ttl, expiry = calculateExpiry(drr)
	}
	switch t := drr.(type) {
	case *dns.SOA:
		return RR{toLowerFQDN(t.Hdr.Name), "SOA", toLowerFQDN(t.Ns), ttl, expiry}, true
	case *dns.NS:
		return RR{toLowerFQDN(t.Hdr.Name), "NS", toLowerFQDN(t.Ns), ttl, expiry}, true
	case *dns.CNAME:
		return RR{toLowerFQDN(t.Hdr.Name), "CNAME", toLowerFQDN(t.Target), ttl, expiry}, true
	case *dns.A:
		return RR{toLowerFQDN(t.Hdr.Name), "A", t.A.String(), ttl, expiry}, true
	case *dns.AAAA:
		return RR{toLowerFQDN(t.Hdr.Name), "AAAA", t.AAAA.String(), ttl, expiry}, true
	case *dns.TXT:
		return RR{toLowerFQDN(t.Hdr.Name), "TXT", strings.Join(t.Txt, "\t"), ttl, expiry}, true
	default:
		fields := strings.Fields(drr.String())
		if len(fields) >= 4 {
			return RR{toLowerFQDN(fields[0]), fields[3], strings.Join(fields[4:], "\t"), ttl, expiry}, true
		}
	}
	return RR{}, false
}

// calculateExpiry calculates the expiry time of an RR.
func calculateExpiry(drr dns.RR) (time.Duration, time.Time) {
	ttl := time.Second * time.Duration(drr.Header().Ttl)
	expiry := time.Now().Add(ttl)
	return ttl, expiry
}
