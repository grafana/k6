package master

// All registered master processors.
var GlobalProcessors []func(*Master) Processor

// Register a master handler.
func RegisterProcessor(factory func(*Master) Processor) {
	GlobalProcessors = append(GlobalProcessors, factory)
}
