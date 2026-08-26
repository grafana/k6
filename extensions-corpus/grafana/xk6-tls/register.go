package tls

import "go.k6.io/k6/v2/js/modules"

func init() {
	modules.Register("k6/x/tls", New())
}
