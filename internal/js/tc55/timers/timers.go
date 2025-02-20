// Package timers is implementing setInterval setTimeout and co.
package timers

import (
	"fmt"
	"slices"
	"time"

	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"github.com/sirupsen/logrus"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type timers struct {
	vu modules.VU

	timerIDCounter uint64

	timers map[uint64]time.Time
	// Maybe in the future if this moves to core it will be expanded to have multiple queues
	queue *timerQueue

	// this used predominantly to get around very unlikely race conditions as we are adding stuff to the event loop
	// from outside of it on multitple timers. And it is easier to just use this then redo half the work it does
	// to make that safe
	taskQueue *taskqueue.TaskQueue
	// used to synchronize around context closing
	taskQueueCh chan struct{}
}

const (
	setTimeoutName  = "setTimeout"
	setIntervalName = "setInterval"
)

// SetupGlobally setups implementatins of setTimeout, clearTimeout, setInterval and clearInterval
// to be accessible globally by setting them on globalThis.
func SetupGlobally(vu modules.VU) error {
	return newTimers(vu).setupGlobally()
}

func newTimers(vu modules.VU) *timers {
	return &timers{
		vu:     vu,
		timers: make(map[uint64]time.Time),
		queue:  new(timerQueue),
	}
}

func (e *timers) setupGlobally() error {
	mapping := map[string]any{
		"setTimeout":    e.setTimeout,
		"clearTimeout":  e.clearTimeout,
		"setInterval":   e.setInterval,
		"clearInterval": e.clearInterval,
	}
	rt := e.vu.Runtime()
	for k, v := range mapping {
		err := rt.Set(k, v)
		if err != nil {
			return fmt.Errorf("error setting up %q globally: %w", k, err)
		}
	}
	return nil
}

func (e *timers) nextID() uint64 {
	e.timerIDCounter++
	return e.timerIDCounter
}

func (e *timers) call(callback sobek.Callable, args []sobek.Value) error {
	// TODO: investigate, not sure GlobalObject() is always the correct value for `this`?
	_, err := callback(e.vu.Runtime().GlobalObject(), args...)
	return err
}

func (e *timers) setTimeout(callback sobek.Callable, delay float64, args ...sobek.Value) uint64 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, false, id)
	return id
}

func (e *timers) clearTimeout(id uint64) {
	_, exists := e.timers[id]
	if !exists {
		return
	}
	delete(e.timers, id)

	e.queue.remove(id)
	e.freeEventLoopIfPossible()
}

func (e *timers) freeEventLoopIfPossible() {
	if e.queue.length() == 0 && e.taskQueue != nil {
		e.closeTaskQueue()
	}
}

func (e *timers) setInterval(callback sobek.Callable, delay float64, args ...sobek.Value) uint64 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, true, id)
	return id
}

func (e *timers) clearInterval(id uint64) {
	e.clearTimeout(id)
}

// https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html#timer-initialisation-steps
// NOTE: previousId from the specification is always send and it is basically id
func (e *timers) timerInitialization(
	callback sobek.Callable, timeout float64, args []sobek.Value, repeat bool, id uint64,
) {
	// skip all the nesting stuff as we do not care about them
	if timeout < 0 {
		timeout = 0
	}

	name := setTimeoutName
	if repeat {
		name = setIntervalName
	}

	if callback == nil {
		common.Throw(e.vu.Runtime(), fmt.Errorf("%s's callback isn't a callable function", name))
	}

	task := func() error {
		// Specification 8.1: If id does not exist in global's map of active timers, then abort these steps.
		if _, exist := e.timers[id]; !exist {
			return nil
		}

		err := e.call(callback, args)

		if _, exist := e.timers[id]; !exist { // 8.4
			return err
		}

		if repeat {
			e.timerInitialization(callback, timeout, args, repeat, id)
		} else {
			delete(e.timers, id)
		}

		return err
	}

	e.runAfterTimeout(&timer{
		id:          id,
		task:        task,
		nextTrigger: time.Now().Add(time.Duration(timeout * float64(time.Millisecond))),
		name:        name,
	})
}

// https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html#run-steps-after-a-timeout
// Notes: this just takes timers as makes the implementation way easier and we do not currently need
// most of the functionality provided
func (e *timers) runAfterTimeout(t *timer) {
	e.timers[t.id] = t.nextTrigger

	// as we have only one orderingId we have one queue
	index := e.queue.add(t)

	if index != 0 {
		return // not a timer at the very beginning
	}

	e.setupTaskTimeout()
}

func (e *timers) runFirstTask() error {
	t := e.queue.pop()
	if t == nil {
		return nil // everything was cleared
	}

	err := t.task()

	if e.queue.length() > 0 {
		e.setupTaskTimeout()
	} else {
		e.freeEventLoopIfPossible()
	}

	return err
}

func (e *timers) setupTaskTimeout() {
	e.queue.stopTimer()
	delay := -time.Since(e.timers[e.queue.first().id])
	if e.taskQueue == nil {
		e.taskQueue = taskqueue.New(e.vu.RegisterCallback)
		e.setupTaskQueueCloserOnIterationEnd()
	}
	q := e.taskQueue
	e.queue.head = time.AfterFunc(delay, func() {
		q.Queue(e.runFirstTask)
	})
}

func (e *timers) closeTaskQueue() {
	// this only runs on the event loop
	if e.taskQueueCh == nil {
		return
	}
	ch := e.taskQueueCh
	// so that we do not execute it twice
	e.taskQueueCh = nil

	select {
	case ch <- struct{}{}:
		// wait for this to happen so we don't need to hit the event loop again
		// instead this just closes the queue
		<-ch
	case <-e.vu.Context().Done(): // still shortcircuit if the context is done as we might block otherwise
	}
}

// logger is helper to get a logger either from the state or the initenv
func (e *timers) logger() logrus.FieldLogger {
	if state := e.vu.State(); state != nil {
		return state.Logger
	}
	return e.vu.InitEnv().Logger
}

func (e *timers) setupTaskQueueCloserOnIterationEnd() {
	ctx := e.vu.Context()
	q := e.taskQueue
	ch := make(chan struct{})
	e.taskQueueCh = ch
	go func() {
		select { // wait for one of the two
		case <-ctx.Done():
			// lets report timers won't be executed and clean the fields for the next execution
			// we need to do this on the event loop as we don't want to have a race
			q.Queue(func() error {
				logger := e.logger()
				for _, timer := range e.queue.queue {
					logger.Warnf("%s %d was stopped because the VU iteration was interrupted",
						timer.name, timer.id)
				}

				clear(e.timers)
				e.queue.stopTimer()
				e.queue = new(timerQueue)
				e.taskQueue = nil
				return nil
			})
			q.Close()
		case <-ch:
			e.timers = make(map[uint64]time.Time)
			e.queue.stopTimer()
			e.queue = new(timerQueue)
			e.taskQueue = nil
			q.Close()
			close(ch)
		}
	}()
}

// this is just a small struct to keep the internals of a timer
type timer struct {
	id          uint64
	nextTrigger time.Time
	task        func() error
	name        string
}

// this is just a list of timers that should be ordered once after the other
// this mostly just has methods to work on the slice
type timerQueue struct {
	queue []*timer
	head  *time.Timer
}

func (tq *timerQueue) add(t *timer) int {
	i := slices.IndexFunc(tq.queue, func(tt *timer) bool {
		return tt.nextTrigger.After(t.nextTrigger)
	})
	if i < 0 {
		i = len(tq.queue)
	}
	tq.queue = slices.Insert(tq.queue, i, t)
	return i
}

func (tq *timerQueue) stopTimer() {
	if tq.head != nil && tq.head.Stop() { // we have a timer and we stopped it before it was over.
		select {
		case <-tq.head.C:
		default:
		}
	}
}

func (tq *timerQueue) remove(id uint64) {
	tq.queue = slices.DeleteFunc(tq.queue, func(t *timer) bool {
		return id == t.id
	})
}

func (tq *timerQueue) pop() *timer {
	length := len(tq.queue)
	if length == 0 {
		return nil
	}
	t := tq.queue[0]
	tq.queue = slices.Delete(tq.queue, 0, 1)
	return t
}

func (tq *timerQueue) length() int {
	return len(tq.queue)
}

func (tq *timerQueue) first() *timer {
	if tq.length() == 0 {
		return nil
	}
	return tq.queue[0]
}
