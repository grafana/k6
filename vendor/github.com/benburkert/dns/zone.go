package dns

import (
	"context"
	"strings"
	"time"
)

// RRSet is a set of resource records indexed by name and type.
type RRSet map[string]map[Type][]Record

// Zone is a contiguous set DNS records under an origin domain name.
type Zone struct {
	Origin string
	TTL    time.Duration

	SOA *SOA

	RRs RRSet
}

// ServeDNS answers DNS queries in zone z.
func (z *Zone) ServeDNS(ctx context.Context, w MessageWriter, r *Query) {
	w.Authoritative(true)

	var found bool
	for _, q := range r.Questions {
		if !strings.HasSuffix(q.Name, z.Origin) {
			continue
		}
		if q.Type == TypeSOA && q.Name == z.Origin {
			w.Answer(q.Name, z.TTL, z.SOA)
			found = true

			continue
		}

		dn := q.Name[:len(q.Name)-len(z.Origin)-1]

		rrs, ok := z.RRs[dn]
		if !ok {
			continue
		}

		for _, rr := range rrs[q.Type] {
			w.Answer(q.Name, z.TTL, rr)
			found = true

			if r.RecursionDesired && rr.Type() == TypeCNAME {
				name := rr.(*CNAME).CNAME
				dn := name[:len(name)-len(z.Origin)-1]

				if rrs, ok := z.RRs[dn]; ok {
					for _, rr := range rrs[q.Type] {
						w.Answer(name, z.TTL, rr)
					}
				}
			}
		}
	}

	if !found {
		w.Status(NXDomain)

		if z.SOA != nil {
			w.Authority(z.Origin, z.TTL, z.SOA)
		}
	}
}
