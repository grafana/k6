package master

import (
	"github.com/loadimpact/speedboat/message"
	"sync"
)

type Processor interface {
	Process(msg message.Message) <-chan message.Message
}

func Process(processors []Processor, msg message.Message) <-chan message.Message {
	ch := make(chan message.Message)
	wg := sync.WaitGroup{}

	// Dispatch processing across a number of processors, using a WaitGroup to record the
	// completion of each one
	for _, processor := range processors {
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
