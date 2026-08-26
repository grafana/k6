// Package icmp contains the xk6-icmp extension.
package icmp

import (
	"github.com/grafana/xk6-icmp/icmp"
	"go.k6.io/k6/v2/js/modules"
)

func init() {
	modules.Register(icmp.ImportPath, icmp.New())
}
