// Package mqtt contains the xk6-mqtt extension.
package mqtt

import (
	"github.com/grafana/xk6-mqtt/mqtt"
	"go.k6.io/k6/v2/js/modules"
)

func init() {
	modules.Register(mqtt.ImportPath, mqtt.New())
}
