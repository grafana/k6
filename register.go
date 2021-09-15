package prometheus

import (
	"github.com/grafana/xk6-output-prometheus-remote/pkg/writer"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("output-prometheus-remote", New)
}

func New(p output.Params) (output.Output, error) {
	return writer.New(p), nil
}
