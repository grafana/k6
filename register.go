// Package template registers the extension for output
package template

import (
	"github.com/grafana/xk6-output-opentelemetry/pkg/opentelemetry"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("xk6-opentelemetry", func(p output.Params) (output.Output, error) {
		return opentelemetry.New(p)
	})
}
