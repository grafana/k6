package remotewrite

import (
	"github.com/grafana/xk6-output-prometheus-remote/pkg/remotewrite"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("output-prometheus-remote", func(p output.Params) (output.Output, error) {
		return remotewrite.New(p)
	})
}
