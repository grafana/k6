package executor

import (
	"context"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/ui/pb"
)

// BaseExecutor is a helper struct that contains common properties and methods
// between most executors. It is intended to be used as an anonymous struct
// inside of most of the executors, for the purpose of reducing boilerplate
// code.
type BaseExecutor struct {
	config         lib.ExecutorConfig
	executionState *lib.ExecutionState
	iterSegIndexMx *sync.Mutex
	iterSegIndex   *lib.SegmentedIndex
	logger         *logrus.Entry
	progress       *pb.ProgressBar
}

// NewBaseExecutor returns an initialized BaseExecutor
func NewBaseExecutor(config lib.ExecutorConfig, es *lib.ExecutionState, logger *logrus.Entry) *BaseExecutor {
	segIdx := lib.NewSegmentedIndex(es.ExecutionTuple)
	return &BaseExecutor{
		config:         config,
		executionState: es,
		logger:         logger,
		iterSegIndexMx: new(sync.Mutex),
		iterSegIndex:   segIdx,
		progress: pb.New(
			pb.WithLeft(config.GetName),
			pb.WithLogger(logger),
		),
	}
}

// nextIterationCounters next scaled(local) and unscaled(global) iteration counters
func (bs *BaseExecutor) nextIterationCounters() (uint64, uint64) {
	bs.iterSegIndexMx.Lock()
	defer bs.iterSegIndexMx.Unlock()
	scaled, unscaled := bs.iterSegIndex.Next()
	return uint64(scaled - 1), uint64(unscaled - 1)
}

// Init doesn't do anything for most executors, since initialization of all
// planned VUs is handled by the executor.
func (bs *BaseExecutor) Init(_ context.Context) error {
	return nil
}

// GetConfig returns the configuration with which this executor was launched.
func (bs *BaseExecutor) GetConfig() lib.ExecutorConfig {
	return bs.config
}

// GetLogger returns the executor logger entry.
func (bs *BaseExecutor) GetLogger() *logrus.Entry {
	return bs.logger
}

// GetProgress just returns the progressbar pointer.
func (bs *BaseExecutor) GetProgress() *pb.ProgressBar {
	return bs.progress
}

// getMetricTags returns a tag set that can be used to emit metrics by the
// executor. The VU ID is optional.
func (bs *BaseExecutor) getMetricTags(vuID *uint64) *metrics.SampleTags {
	tags := make(map[string]string, len(bs.executionState.Test.Options.RunTags))
	for k, v := range bs.executionState.Test.Options.RunTags {
		tags[k] = v
	}
	if bs.executionState.Test.Options.SystemTags.Has(metrics.TagScenario) {
		tags["scenario"] = bs.config.GetName()
	}
	if vuID != nil && bs.executionState.Test.Options.SystemTags.Has(metrics.TagVU) {
		tags["vu"] = strconv.FormatUint(*vuID, 10)
	}
	return metrics.IntoSampleTags(&tags)
}
