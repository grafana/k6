// Package eventloop implements an event loop to be used thought js and it's subpackages
package eventloop

import (
	"fmt"
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
)

// EventLoop implements an event with
// handling of unhandled rejected promises.
//
// A specific thing about this event loop is that it will wait to return
// not only until the queue is empty but until nothing is registered that it will run in the future.
// This is in contrast with more common behaviours where it only returns on
// a specific event/action or when the loop is empty.
// This is required as in k6 iterations (for which event loop will be primary used)
// are supposed to be independent and any work started in them needs to finish,
// but also they need to end when all the instructions are done.
// Additionally because of this on any error while the event loop will exit it's
// required to wait on the event loop to be empty before the execution can continue.
type EventLoop struct {
	lock                sync.Mutex
	queue               []func() error
	wakeupCh            chan struct{} // TODO: maybe use sync.Cond ?
	registeredCallbacks int
	vu                  modules.VU

	// pendingPromiseRejections are rejected promises with no handler,
	// if there is something in this map at an end of an event loop then it will exit with an error.
	// It's similar to what Deno and Node do.
	pendingPromiseRejections map[*sobek.Promise]struct{}
}

// New returns a new event loop with a few helpers attached to it:
// - adding setTimeout javascript implementation
// - reporting (and aborting on) unhandled promise rejections
func New(vu modules.VU) *EventLoop {
	e := &EventLoop{
		wakeupCh:                 make(chan struct{}, 1),
		pendingPromiseRejections: make(map[*sobek.Promise]struct{}),
		vu:                       vu,
	}
	vu.Runtime().SetPromiseRejectionTracker(e.promiseRejectionTracker)

	return e
}

func (e *EventLoop) wakeup() {
	select {
	case e.wakeupCh <- struct{}{}:
	default:
	}
}

// RegisterCallback signals to the event loop that you are going to do some
// asynchronous work off the main thread and that you may need to execute some
// code back on the main thread when you are done. So, once you call this
// method, the event loop will wait for you to finish and give it the callback
// it needs to run back on the main thread before it can end the whole current
// script iteration.
//
// RegisterCallback() *must* be called from the main runtime thread, but its
// result enqueueCallback() is thread-safe and can be called from any goroutine.
// enqueueCallback() ensures that its callback parameter is added to the VU
// runtime's tasks queue, to be executed on the main runtime thread eventually,
// when the VU is done with the other tasks before it. Unless the whole event
// loop has been stopped, invoking enqueueCallback() will queue its argument and
// "wake up" the loop (if it was idle, but not stopped).
//
// Keep in mind that once you call RegisterCallback(), you *must* also call
// enqueueCallback() exactly once, even if don't actually need to run any code
// on the main thread. If that's the case, you can pass an empty no-op callback
// to it, but you must call it! The event loop will wait for the
// enqueueCallback() invocation and the k6 iteration won't finish and will be
// stuck until the VU itself has been stopped (e.g. because the whole test or
// scenario has ended). Any error returned by any callback on the main thread
// will abort the current iteration and no further event loop callbacks will be
// executed in the same iteration.
//
// A common pattern for async work is something like this:
//
//	func doAsyncWork(vu modules.VU) *sobek.Promise {
//	    enqueueCallback := vu.RegisterCallback()
//	    p, resolve, reject := vu.Runtime().NewPromise()
//
//	    // Do the actual async work in a new independent goroutine, but make
//	    // sure that the Promise resolution is done on the main thread:
//	    go func() {
//	        // Also make sure to abort early if the context is cancelled, so
//	        // the VU is not stuck when the scenario ends or Ctrl+C is used:
//	        result, err := doTheActualAsyncWork(vu.Context())
//	        enqueueCallback(func() error {
//	            if err != nil {
//	                reject(err)
//	            } else {
//	                resolve(result)
//	            }
//	            return nil  // do not abort the iteration
//	        })
//	    }()
//
//	    return p
//	}
//
// This ensures that the actual work happens asynchronously, while the Promise
// is immediately returned and the main thread resumes execution. It also
// ensures that the Promise resolution happens safely back on the main thread
// once the async work is done, as required by Sobek and all other JS runtimes.
//
// TODO: rename to ReservePendingCallback or something more appropriate?
func (e *EventLoop) RegisterCallback() (enqueueCallback func(func() error)) {
	e.lock.Lock()
	var callbackCalled bool
	e.registeredCallbacks++
	e.lock.Unlock()

	return func(f func() error) {
		e.lock.Lock()
		if callbackCalled { // this is protected by the lock on the event loop
			e.lock.Unlock() // let not lock up the whole event loop, somebody could recover from the panic
			panic("RegisterCallback called twice")
		}
		callbackCalled = true
		e.queue = append(e.queue, f)
		e.registeredCallbacks--
		e.lock.Unlock()
		e.wakeup()
	}
}

func (e *EventLoop) promiseRejectionTracker(p *sobek.Promise, op sobek.PromiseRejectionOperation) {
	// No locking necessary here as the Sobek runtime will call this synchronously
	// Read Notes on https://tc39.es/ecma262/#sec-host-promise-rejection-tracker
	if op == sobek.PromiseRejectionReject {
		e.pendingPromiseRejections[p] = struct{}{}
	} else { // sobek.PromiseRejectionHandle so a promise that was previously rejected without handler now got one
		delete(e.pendingPromiseRejections, p)
	}
}

func (e *EventLoop) popAll() (queue []func() error, awaiting bool) {
	e.lock.Lock()
	queue = e.queue
	e.queue = make([]func() error, 0, len(queue))
	awaiting = e.registeredCallbacks != 0
	e.lock.Unlock()
	return
}

func (e *EventLoop) putInfront(queue []func() error) {
	e.lock.Lock()
	e.queue = append(queue, e.queue...)
	e.lock.Unlock()
}

// Start will run the event loop until it's empty and there are no uninvoked registered callbacks
// or a queued function returns an error. The provided firstCallback will be the first thing executed.
// After Start returns the event loop can be reused as long as waitOnRegistered is called.
func (e *EventLoop) Start(firstCallback func() error) error {
	e.pendingPromiseRejections = make(map[*sobek.Promise]struct{})
	e.queue = []func() error{firstCallback}
	for {
		queue, awaiting := e.popAll()

		if len(queue) == 0 {
			if !awaiting {
				return nil
			}
			<-e.wakeupCh
			continue
		}

		for i, f := range queue {
			if err := f(); err != nil {
				e.putInfront(queue[i+1:])

				return err
			}
		}

		// This will get a random unhandled rejection instead of the first one, for example.
		// But that seems to be the case in other tools as well so it seems to not be that big of a problem.
		for promise := range e.pendingPromiseRejections {
			value := promise.Result()
			if !sobek.IsNull(value) && !sobek.IsUndefined(value) {
				if o := value.ToObject(e.vu.Runtime()); o != nil {
					if stack := o.Get("stack"); stack != nil {
						value = stack
					}
				}
			}
			// this is the de facto wording in both firefox and deno at least
			return fmt.Errorf("Uncaught (in promise) %s", value) //nolint:stylecheck
		}
	}
}

// WaitOnRegistered waits on all registered callbacks so we know nothing is still doing work.
// This does call back the callbacks and more can be queued over time.
// A different mechanism needs to be used to tell the users that the event loop has errored out or winding down for a
// different reason.
func (e *EventLoop) WaitOnRegistered() {
	for {
		queue, awaiting := e.popAll()
		if len(queue) == 0 {
			if !awaiting {
				return
			}
			<-e.wakeupCh
			continue
		}

		for _, f := range queue {
			if err := f(); err != nil {
				// TODO figure out if we should buffer all errors happening or send them on a channel
				continue
			}
		}
	}
}
