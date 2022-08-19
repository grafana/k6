// Package timers is implementing setInterval setTimeout and co. Not to be used mostly for testing purposes
package timers

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct{}

// Timers represents an instance of the timers module.
type Timers struct {
	vu modules.VU

	timerStopCounter uint32
	timerStopsLock   sync.Mutex
	timerStops       map[uint32]chan struct{}
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Timers{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Timers{
		vu:         vu,
		timerStops: make(map[uint32]chan struct{}),
	}
}

// Exports returns the exports of the k6 module.
func (e *Timers) Exports() modules.Exports {
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

func (e *Timers) getTimerStopCh() (uint32, chan struct{}) {
	id := atomic.AddUint32(&e.timerStopCounter, 1)
	ch := make(chan struct{})
	e.timerStopsLock.Lock()
	e.timerStops[id] = ch
	e.timerStopsLock.Unlock()
	return id, ch
}

func (e *Timers) stopTimerCh(id uint32) bool { //nolint:unparam
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

func (e *Timers) call(callback goja.Callable, args []goja.Value) error {
	// TODO: investigate, not sure GlobalObject() is always the correct value for `this`?
	_, err := callback(e.vu.Runtime().GlobalObject(), args...)
	return err
}

func (e *Timers) setTimeout(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	runOnLoop := e.vu.RegisterCallback()
	id, stopCh := e.getTimerStopCh()

	if delay < 0 {
		delay = 0
	}

	go func() {
		timer := time.NewTimer(time.Duration(delay * float64(time.Millisecond)))
		defer func() {
			timer.Stop()
			e.stopTimerCh(id)
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

func (e *Timers) clearTimeout(id uint32) {
	e.stopTimerCh(id)
}

func (e *Timers) setInterval(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	tq := taskqueue.New(e.vu.RegisterCallback)
	id, stopCh := e.getTimerStopCh()

	go func() {
		ticker := time.NewTicker(time.Duration(delay * float64(time.Millisecond)))
		defer func() {
			e.stopTimerCh(id)
			ticker.Stop()
		}()

		for {
			defer tq.Close()
			select {
			case <-ticker.C:
				tq.Queue(func() error {
					return e.call(callback, args)
				})
			case <-stopCh:
				return
			case <-e.vu.Context().Done():
				e.vu.State().Logger.Warnf("setInterval %d was stopped because the VU iteration was interrupted", id)
				return
			}
		}
	}()

	return id
}

func (e *Timers) clearInterval(id uint32) {
	e.stopTimerCh(id)
}
