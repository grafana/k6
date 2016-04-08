package worker

import (
	"github.com/loadimpact/speedboat/comm"
	"testing"
)

func TestRegisterProcessor(t *testing.T) {
	oldGlobalProcessors := GlobalProcessors
	GlobalProcessors = nil
	defer func() { GlobalProcessors = oldGlobalProcessors }()

	RegisterProcessor(func(w *Worker) comm.Processor { return nil })
	if len(GlobalProcessors) != 1 {
		t.Error("Processor not registered")
	}
}
