// Package expv2 contains a Cloud output using a Protobuf
// binary format for encoding payloads.
package expv2

import (
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/metrics"

	"github.com/sirupsen/logrus"
)

// Output sends result data to the k6 Cloud service.
type Output struct {
	logger       logrus.FieldLogger
	config       cloudapi.Config
	referenceID  string
	testStopFunc func(error)
}

// New creates a new cloud output.
func New(logger logrus.FieldLogger, conf cloudapi.Config) (*Output, error) {
	return &Output{
		config: conf,
		logger: logger.WithFields(logrus.Fields{"output": "cloudv2"}),
	}, nil
}

// SetReferenceID sets the Cloud's test run ID.
func (o *Output) SetReferenceID(refID string) {
	o.referenceID = refID
}

// SetTestRunStopCallback receives the function that
// that stops the engine when it is called.
// It should be called on critical errors.
func (o *Output) SetTestRunStopCallback(stopFunc func(error)) {
	o.testStopFunc = stopFunc
}

// Start starts the goroutine that would listen
// for metric samples and send them to the cloud.
func (o *Output) Start() error {
	o.logger.Debug("Starting...")

	// TODO: add start implementation

	o.logger.Debug("Started!")
	return nil
}

// StopWithTestError gracefully stops all metric emission from the output.
func (o *Output) StopWithTestError(testErr error) error {
	o.logger.Debug("Stopping...")

	// TODO: add stop implementation

	o.logger.Debug("Stopped!")
	return nil
}

// AddMetricSamples receives the samples streaming.
func (o *Output) AddMetricSamples(s []metrics.SampleContainer) {
	// TODO: implement
}
