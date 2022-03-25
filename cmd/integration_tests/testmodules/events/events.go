// Package events implements setInterval, setTimeout and co. Not to be used, mostly for testing purposes
package events

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct{}

// Events represents an instance of the events module.
type Events struct {
	vu modules.VU

	timerStopCounter uint32
	timerStopsLock   sync.Mutex
	timerStops       map[uint32]chan struct{}
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Events{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Events{
		vu:         vu,
		timerStops: make(map[uint32]chan struct{}),
	}
}

// Exports returns the exports of the k6 module.
func (e *Events) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"setTimeout":    e.setTimeout,
			"clearTimeout":  e.clearTimeout,
			"setInterval":   e.setInterval,
			"clearInterval": e.clearInterval,
		},
	}
}

func noop() error { return nil }

func (e *Events) getTimerStopCh() (uint32, chan struct{}) {
	id := atomic.AddUint32(&e.timerStopCounter, 1)
	ch := make(chan struct{})
	e.timerStopsLock.Lock()
	e.timerStops[id] = ch
	e.timerStopsLock.Unlock()
	return id, ch
}

func (e *Events) stopTimerCh(id uint32) bool { //nolint:unparam
	e.timerStopsLock.Lock()
	defer e.timerStopsLock.Unlock()
	ch, ok := e.timerStops[id]
	if !ok {
		return false
	}
	delete(e.timerStops, id)
	close(ch)
	return true
}

func (e *Events) call(callback goja.Callable, args []goja.Value) error {
	// TODO: investigate, not sure GlobalObject() is always the correct value for `this`?
	_, err := callback(e.vu.Runtime().GlobalObject(), args...)
	return err
}

func (e *Events) setTimeout(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	runOnLoop := e.vu.RegisterCallback()
	id, stopCh := e.getTimerStopCh()

	if delay < 0 {
		delay = 0
	}

	go func() {
		timer := time.NewTimer(time.Duration(delay * float64(time.Millisecond)))
		defer func() {
			e.stopTimerCh(id)
			if !timer.Stop() {
				<-timer.C
			}
		}()

		select {
		case <-timer.C:
			runOnLoop(func() error {
				return e.call(callback, args)
			})
		case <-stopCh:
			runOnLoop(noop)
		case <-e.vu.Context().Done():
			e.vu.State().Logger.Warnf("setTimeout %d was stopped because the VU iteration was interrupted", id)
			runOnLoop(noop)
		}
	}()

	return id
}

func (e *Events) clearTimeout(id uint32) {
	e.stopTimerCh(id)
}

func (e *Events) setInterval(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	runOnLoop := e.vu.RegisterCallback()
	id, stopCh := e.getTimerStopCh()

	go func() {
		ticker := time.NewTicker(time.Duration(delay * float64(time.Millisecond)))
		defer func() {
			e.stopTimerCh(id)
			ticker.Stop()
		}()

		for {
			select {
			case <-ticker.C:
				runOnLoop(func() error {
					runOnLoop = e.vu.RegisterCallback()
					return e.call(callback, args)
				})
			case <-stopCh:
				runOnLoop(noop)
				return
			case <-e.vu.Context().Done():
				e.vu.State().Logger.Warnf("setInterval %d was stopped because the VU iteration was interrupted", id)
				runOnLoop(noop)
				return
			}
		}
	}()

	return id
}

func (e *Events) clearInterval(id uint32) {
	e.stopTimerCh(id)
}
