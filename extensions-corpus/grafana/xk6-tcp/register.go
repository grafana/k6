// Package tcp contains the xk6-tcp k6 extension.
package tcp

import (
	"github.com/grafana/xk6-tcp/tcp"
	"go.k6.io/k6/v2/js/modules"
)

func init() {
	modules.Register(tcp.ImportPath, tcp.New())
}
