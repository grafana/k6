package worker

import (
	"github.com/loadimpact/speedboat/master"
)

// All registered worker processors.
var GlobalProcessors []func(*Worker) master.Processor

// Register a worker processor.
func RegisterProcessor(factory func(*Worker) master.Processor) {
	GlobalProcessors = append(GlobalProcessors, factory)
}
