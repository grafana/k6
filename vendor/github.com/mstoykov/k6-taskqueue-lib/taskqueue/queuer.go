// Package taskqueue implements a callback queuer to be used with the event loop in k6
package taskqueue

import (
	"sync"
)

// TaskQueue makes it easy to queue multiple callback to the k6 event loop.
// This in particular works around the problem that in order to queue a callback you need to be on the event loop
// But if you already use the callback you need to wait for the queued callback to be executed to "reset"
// the callback while it's executing on the event loop
type TaskQueue struct {
	callback         func(Task)
	registerCallback func() func(Task)
	queue            []Task // TODO use a ring?
	m                sync.Mutex
	closed           bool
}

// Task is just a representation of a task that will be added to the event loop
type Task func() error

// New returns a new CallbackQueuer that will use the provide registrerCallback function
func New(registerCallback func() func(func() error)) *TaskQueue {
	// this is to work around the fact that RegistrerCallback doesn't know about Task
	// related to https://github.com/golang/go/issues/8082
	internal := func() func(Task) {
		f := registerCallback()
		return func(t Task) {
			f(t)
		}
	}
	tq := &TaskQueue{
		callback:         internal(),
		registerCallback: internal,
	}
	return tq
}

// Close will stop the queue letting the event loop finish, and is required to be called for that reason
func (tq *TaskQueue) Close() {
	tq.m.Lock()
	defer tq.m.Unlock()
	if tq.closed {
		return
	}
	if tq.callback == nil { // already something queued
		tq.queue = append(tq.queue, func() error {
			tq.Close()
			return nil
		})
		return
	}

	tq.closed = true
	tq.callback(func() error { return nil })
}

// Queue the provided function for execution.
// If used after Close is called it will not actually execute anything or return an error
func (tq *TaskQueue) Queue(t Task) {
	tq.m.Lock()
	defer tq.m.Unlock()
	if tq.closed {
		return
	}
	if tq.callback == nil { // already something queued
		tq.queue = append(tq.queue, t)
		return
	}
	callback := tq.callback
	tq.callback = nil
	callback(func() error { return tq.innerQueueATask(t) })
}

func (tq *TaskQueue) innerQueueATask(t Task) error {
	tq.m.Lock()
	if tq.callback == nil && !tq.closed {
		tq.callback = tq.registerCallback() // refresh
	}
	if len(tq.queue) != 0 {
		for _, newF := range tq.queue { // queue the queue
			newF := newF
			tq.registerCallback()(func() error {
				return tq.innerQueueATask(newF)
			})
		}
		tq.queue = tq.queue[:0]
	}
	tq.m.Unlock() // we actually need to unlock before executing in case that will use the queuer
	// actually execute the function that we need to
	return t()
}
