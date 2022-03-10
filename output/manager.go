package output

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/stats"
)

// TODO: completely get rid of this, see https://github.com/grafana/k6/issues/2430
const sendBatchToOutputsRate = 50 * time.Millisecond

// Manager can be used to manage multiple outputs at the same time.
type Manager struct {
	outputs     []Output
	startedUpTo int // keep track of which outputs are started or stopped
	waitForPump *sync.WaitGroup
	logger      logrus.FieldLogger

	testStopCallback func(error)
}

// NewManager returns a new manager for the given outputs.
func NewManager(outputs []Output, logger logrus.FieldLogger, testStopCallback func(error)) *Manager {
	return &Manager{
		outputs:          outputs,
		logger:           logger.WithField("component", "output-manager"),
		testStopCallback: testStopCallback,
		waitForPump:      &sync.WaitGroup{},
	}
}

// StartOutputs spins up all configured outputs and then starts a new goroutine
// that pipes metrics from the given samples channel to them. If some output
// fails to start, it stops the already started ones. This may take some time,
// since some outputs make initial network requests to set up whatever remote
// services are going to listen to them. After the
func (om *Manager) Start(samplesChan chan stats.SampleContainer) (wait func(), err error) {
	if err := om.startOutputs(); err != nil {
		return nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	sendToOutputs := func(sampleContainers []stats.SampleContainer) {
		for _, out := range om.outputs {
			out.AddMetricSamples(sampleContainers)
		}
	}

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(sendBatchToOutputsRate)
		defer ticker.Stop()

		buffer := make([]stats.SampleContainer, 0, cap(samplesChan))
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
				buffer = make([]stats.SampleContainer, 0, cap(buffer))
			}
		}
	}()

	return wg.Wait, nil
}

func (om *Manager) startOutputs() error {
	om.logger.Debugf("Starting %d outputs...", len(om.outputs))
	for _, out := range om.outputs {
		if stopOut, ok := out.(WithTestRunStop); ok {
			stopOut.SetTestRunStopCallback(om.testStopCallback)
		}

		if err := out.Start(); err != nil {
			om.StopOutputs()
			return err
		}
		om.startedUpTo++
	}
	return nil
}

// StopOutputs stops all already started outputs. We keep track so we don't
// accidentally stop an output twice.
func (om *Manager) StopOutputs() {
	om.logger.Debugf("Stopping %d outputs...", om.startedUpTo)
	for ; om.startedUpTo > 0; om.startedUpTo-- {
		out := om.outputs[om.startedUpTo-1]
		if err := out.Stop(); err != nil {
			om.logger.WithError(err).Errorf("Stopping output '%s' (%d) failed", out.Description(), om.startedUpTo-1)
		}
	}
}

// SetRunStatus checks which outputs implement the WithRunStatusUpdates
// interface and sets the provided RunStatus to them.
func (om *Manager) SetRunStatus(status lib.RunStatus) {
	for _, out := range om.outputs {
		if statUpdOut, ok := out.(WithRunStatusUpdates); ok {
			statUpdOut.SetRunStatus(status)
		}
	}
}
