// Package mqtt contains the xk6-mqtt extension.
package mqtt

import (
	"github.com/grafana/xk6-mqtt/mqtt"
	extensionapi "go.k6.io/k6-extension-api"
)

func init() {
	extensionapi.Register(mqtt.ImportPath, mqtt.New())
}
