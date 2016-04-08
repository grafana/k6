package comm

import (
	"sync"
)

type Processor interface {
	Process(msg Message) <-chan Message
}

func Process(processors []Processor, msg Message) <-chan Message {
	ch := make(chan Message)
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
