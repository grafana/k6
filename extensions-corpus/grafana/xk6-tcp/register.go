// Package tcp contains the xk6-tcp k6 extension.
package tcp

import (
	"github.com/grafana/xk6-tcp/tcp"
	extensionapi "go.k6.io/k6-extension-api"
)

func init() {
	extensionapi.Register(tcp.ImportPath, tcp.New())
}
