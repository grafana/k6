package dnsr

import (
	"strings"

	"github.com/miekg/dns"
)

func parent(name string) (string, bool) {
	labels := dns.SplitDomainName(name)
	if labels == nil {
		return "", false
	}
	return toLowerFQDN(strings.Join(labels[1:], ".")), true
}

func toLowerFQDN(name string) string {
	return dns.Fqdn(strings.ToLower(name))
}
