package k6test

import (
	"github.com/sirupsen/logrus"

	k6lib "go.k6.io/k6/lib"
	k6executor "go.k6.io/k6/lib/executor"
)

// TestExecutor is a k6lib.ExecutorConfig implementation
// for testing purposes.
type TestExecutor struct {
	k6executor.BaseConfig
}

// GetDescription returns a mock Executor description.
func (te *TestExecutor) GetDescription(*k6lib.ExecutionTuple) string {
	return "TestExecutor"
}

// GetExecutionRequirements is a dummy implementation that just returns nil.
func (te *TestExecutor) GetExecutionRequirements(*k6lib.ExecutionTuple) []k6lib.ExecutionStep {
	return nil
}

// NewExecutor is a dummy implementation that just returns nil.
func (te *TestExecutor) NewExecutor(*k6lib.ExecutionState, *logrus.Entry) (k6lib.Executor, error) {
	return nil, nil //nolint:nilnil
}

// HasWork is a dummy implementation that returns true.
func (te *TestExecutor) HasWork(*k6lib.ExecutionTuple) bool {
	return true
}
