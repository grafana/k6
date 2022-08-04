package outputtest

import (
	"strconv"

	"github.com/spf13/afero"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func init() {
	output.RegisterExtension("outputtest", func(params output.Params) (output.Output, error) {
		return &Output{params: params}, nil
	})
}

// Output is meant to test xk6 and the output extension sub-system of k6.
type Output struct {
	params     output.Params
	calcResult float64
	outputFile afero.File
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	return "test output extension"
}

// Start opens the specified output file.
func (o *Output) Start() error {
	out, err := o.params.FS.Create(o.params.ConfigArgument)
	if err != nil {
		return err
	}
	o.outputFile = out

	return nil
}

// AddMetricSamples just plucks out the metric we're interested in.
func (o *Output) AddMetricSamples(sampleContainers []metrics.SampleContainer) {
	for _, sc := range sampleContainers {
		for _, sample := range sc.GetSamples() {
			if sample.Metric.Name == "foos" {
				o.calcResult += sample.Value
			}
		}
	}
}

// Stop saves the dummy results and closes the file.
func (o *Output) Stop() error {
	_, err := o.outputFile.Write([]byte(strconv.FormatFloat(o.calcResult, 'f', 0, 64)))
	if err != nil {
		return err
	}
	return o.outputFile.Close()
}
