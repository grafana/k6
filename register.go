package prometheus

import (
	"github.com/grafana/xk6-output-prometheus-remote/pkg/prometheus"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("output-prometheus-remote", func(p output.Params) (output.Output, error) {
		return prometheus.New(p)
	})
}
