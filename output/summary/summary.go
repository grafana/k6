package summary

import (
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

var _ output.Output = &Output{}

// Output ...
type Output struct {
}

// New returns a new JSON output.
func New(params output.Params) (output.Output, error) {
	return &Output{}, nil
}

func (o Output) Description() string {
	return ""
}

func (o Output) Start() error {
	return nil
}

func (o Output) AddMetricSamples(samples []metrics.SampleContainer) {

}

func (o Output) Stop() error {
	return nil
}
