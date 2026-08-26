// Package dns provides a k6 extension for DNS resolution and lookup.
package dns

import (
	"github.com/grafana/xk6-dns/dns"
	"go.k6.io/k6/v2/js/modules"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/dns".
func init() {
	modules.Register("k6/x/dns", new(dns.RootModule))
}
