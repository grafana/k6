package tls

import extensionapi "go.k6.io/k6-extension-api"

func init() {
	extensionapi.Register("k6/x/tls", New())
}
