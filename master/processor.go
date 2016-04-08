package master

import (
	"github.com/loadimpact/speedboat/comm"
	"sync"
)

type Processor interface {
	Process(msg comm.Message) <-chan comm.Message
}

func Process(processors []Processor, msg comm.Message) <-chan comm.Message {
	ch := make(chan comm.Message)
	wg := sync.WaitGroup{}

	// Dispatch processing across a number of processors, using a WaitGroup to record the
	// completion of each one
	for _, processor := range processors {
		processor := processor
		wg.Add(1)
		go func() {
			// No matter what happens, mark this processor as done once this goroutine returns
			defer wg.Done()

			// Forward resulting messages from the processor
			for m := range processor.Process(msg) {
				ch <- m
			}
		}()
	}

	// Wait on the WaitGroup before closing the channel, signalling that we're done here
	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}
