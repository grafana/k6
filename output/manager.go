package output

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
)

// Manager can be used to manage multiple outputs at the same time.
type Manager struct {
	outputs []Output
	logger  logrus.FieldLogger

	testStopCallback func(error)
}

// NewManager returns a new manager for the given outputs.
func NewManager(outputs []Output, logger logrus.FieldLogger, testStopCallback func(error)) *Manager {
	return &Manager{
		outputs:          outputs,
		logger:           logger.WithField("component", "output-manager"),
		testStopCallback: testStopCallback,
	}
}

// StartOutputs spins up all configured outputs. If some output fails to start,
// it stops the already started ones. This may take some time, since some
// outputs make initial network requests to set up whatever remote services are
// going to listen to them.
func (om *Manager) StartOutputs() error {
	om.logger.Debugf("Starting %d outputs...", len(om.outputs))
	for i, out := range om.outputs {
		if stopOut, ok := out.(WithTestRunStop); ok {
			stopOut.SetTestRunStopCallback(om.testStopCallback)
		}

		if err := out.Start(); err != nil {
			om.stopOutputs(err, i)
			return err
		}
	}
	return nil
}

// StopOutputs stops all configured outputs.
func (om *Manager) StopOutputs(testErr error) {
	om.stopOutputs(testErr, len(om.outputs))
}

func (om *Manager) stopOutputs(testErr error, upToID int) {
	om.logger.Debugf("Stopping %d outputs...", upToID)
	for i := 0; i < upToID; i++ {
		out := om.outputs[i]
		var err error
		if sout, ok := out.(WithStopWithTestError); ok {
			err = sout.StopWithTestError(testErr)
		} else {
			err = out.Stop()
		}

		if err != nil {
			om.logger.WithError(err).Errorf("Stopping output %d failed", i)
		}
	}
}

// AddMetricSamples is a temporary method to make the Manager usable in the
// current Engine. It needs to be replaced with the full metric pump.
//
// TODO: refactor
func (om *Manager) AddMetricSamples(sampleContainers []metrics.SampleContainer) {
	if len(sampleContainers) == 0 {
		return
	}

	for _, out := range om.outputs {
		out.AddMetricSamples(sampleContainers)
	}
}
