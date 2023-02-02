package output

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
)

// TODO: completely get rid of this, see https://github.com/grafana/k6/issues/2430
const sendBatchToOutputsRate = 50 * time.Millisecond

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

// Start spins up all configured outputs and then starts a new goroutine that
// pipes metrics from the given samples channel to them.
//
// If some output fails to start, it stops the already started ones. This may
// take some time, since some outputs make initial network requests to set up
// whatever remote services are going to listen to them.
//
// If all outputs start successfully, this method will return 2 callbacks. The
// first one, wait(), will block until the samples channel has been closed and
// all of its buffered metrics have been sent to all outputs. The second
// callback will call the Stop() or StopWithTestError() method of every output.
func (om *Manager) Start(samplesChan chan metrics.SampleContainer) (wait func(), finish func(error), err error) {
	if err := om.startOutputs(); err != nil {
		return nil, nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	sendToOutputs := func(sampleContainers []metrics.SampleContainer) {
		for _, out := range om.outputs {
			out.AddMetricSamples(sampleContainers)
		}
	}

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(sendBatchToOutputsRate)
		defer ticker.Stop()

		buffer := make([]metrics.SampleContainer, 0, cap(samplesChan))
		for {
			select {
			case sampleContainer, ok := <-samplesChan:
				if !ok {
					sendToOutputs(buffer)
					return
				}
				buffer = append(buffer, sampleContainer)
			case <-ticker.C:
				sendToOutputs(buffer)
				buffer = make([]metrics.SampleContainer, 0, cap(buffer))
			}
		}
	}()

	wait = wg.Wait
	finish = func(testErr error) {
		wait() // just in case, though it shouldn't be needed if API is used correctly
		om.stopOutputs(testErr, len(om.outputs))
	}
	return wait, finish, nil
}

// startOutputs spins up all configured outputs. If some output fails to start,
// it stops the already started ones. This may take some time, since some
// outputs make initial network requests to set up whatever remote services are
// going to listen to them.
func (om *Manager) startOutputs() error {
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
