// Package queuer implements a callback queuer to be used with the new (as of v0.37,0) event loop in k6
package queuer

import "sync"

// TODO  maybe rename the whole thing to taskqueue as that might be better

// CallbackQueuer makes it easy to queue multiple callback to the k6 event loop.
// This in particular works around the problem that in order to queue a callback you need to be on the event loop
// But if you already use the callback you need to wait for the queued callback to be executed to "reset"
// the callback while it's executing on the event loop
type CallbackQueuer struct {
	callback          func(func() error)
	registrerCallback func() func(func() error)
	queue             []func() error // TODO use a ring?
	closed            bool
	m                 sync.Mutex
}

// New returns a new CallbackQueuer that will use the provide registrerCallback function
func New(registrerCallback func() func(func() error)) *CallbackQueuer {
	return &CallbackQueuer{
		callback:          registrerCallback(),
		registrerCallback: registrerCallback,
	}
}

// Close will stop the queue letting the event loop finish, it is required to be called
func (fq *CallbackQueuer) Close() {
	// fmt.Println("queuer close")
	fq.m.Lock()
	defer fq.m.Unlock()
	if fq.closed {
		return
	}
	if fq.callback == nil { // already something queued
		// fmt.Println("callback is nil on close ")
		fq.queue = append(fq.queue, func() error {
			fq.Close()
			return nil
		})
		return
	}

	fq.closed = true
	// fmt.Println("close being executed")
	callback := fq.callback
	fq.callback = nil
	callback(func() error { return nil })
}

// QueueATask queues the provided function for execution. If used after Close is called it will not actually execute.
func (fq *CallbackQueuer) QueueATask(f func() error) {
	fq.m.Lock()
	defer fq.m.Unlock()
	if fq.closed {
		return
	}
	if fq.callback == nil { // already something queued
		fq.queue = append(fq.queue, f)
		return
	}
	callback := fq.callback
	fq.callback = nil
	callback(func() error { return fq.innerQueueATask(f) })
}

func (fq *CallbackQueuer) innerQueueATask(f func() error) error {
	fq.m.Lock()
	if fq.callback == nil && !fq.closed {
		fq.callback = fq.registrerCallback() // refresh
	}
	if len(fq.queue) != 0 {
		// fmt.Println("queue is not empty", len(fq.queue))
		for _, newF := range fq.queue { // queue the queue
			newF := newF
			fq.registrerCallback()(func() error { return fq.innerQueueATask(newF) })
		}
		fq.queue = fq.queue[:0]
		// fmt.Println("queue is now empty", len(fq.queue))
	}
	fq.m.Unlock() // we actually need to unlock before executing in case that will use the queuer
	// actually execute the function that we need to
	return f()
}
