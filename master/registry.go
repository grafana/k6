package master

import (
	"github.com/loadimpact/speedboat/comm"
)

// All registered master processors.
var GlobalProcessors []func(*Master) comm.Processor

// Register a master handler.
func RegisterProcessor(factory func(*Master) comm.Processor) {
	GlobalProcessors = append(GlobalProcessors, factory)
}
