// Package dns provides a k6 extension for DNS resolution and lookup.
package dns

import (
	"github.com/grafana/xk6-dns/dns"
	extensionapi "go.k6.io/k6-extension-api"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/dns".
func init() {
	extensionapi.Register(dns.ImportPath, dns.New())
}
