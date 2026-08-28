// Package xk6kafka is the module entrypoint for the grafana/xk6-kafka k6
// extension. xk6 imports this root package; its init registers the k6/x/kafka
// module. The implementation lives in pkg/kafka, which has no import side
// effects of its own.
package xk6kafka

import (
	extensionapi "go.k6.io/k6-extension-api"

	"github.com/grafana/xk6-kafka/pkg/kafka"
)

func init() {
	extensionapi.Register("k6/x/kafka", new(kafka.RootModule))
}
