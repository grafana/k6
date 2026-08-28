// Package icmp contains the xk6-icmp extension.
package icmp

import (
	"github.com/grafana/xk6-icmp/icmp"
	extensionapi "go.k6.io/k6-extension-api"
)

func init() {
	extensionapi.Register(icmp.ImportPath, icmp.New())
}
