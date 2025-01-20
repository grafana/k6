// Package events is used for testing the event functionality.
package events

import (
	"sync"

	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct {
	initOnce               sync.Once
	globalEvents, vuEvents []event.Type
	// Used by the test function to wait for all event handler goroutines to exit,
	// to avoid dangling goroutines.
	WG sync.WaitGroup
	// Closed by the global event handler once the Exit event is received, and
	// used as a signal for VU event handlers to also exit.
	exit chan struct{}
}

// Events represents an instance of the events module.
type Events struct{}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Events{}
)

// New returns a pointer to a new RootModule instance.
func New(globalEvents, vuEvents []event.Type) *RootModule {
	return &RootModule{
		initOnce:     sync.Once{},
		exit:         make(chan struct{}),
		globalEvents: globalEvents,
		vuEvents:     vuEvents,
	}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rm.initOnce.Do(func() {
		sid, evtCh := vu.Events().Global.Subscribe(rm.globalEvents...)
		logger := vu.InitEnv().Logger
		rm.WG.Add(1)
		go func() {
			defer func() {
				close(rm.exit)
				rm.WG.Done()
			}()
			for {
				select {
				case evt, ok := <-evtCh:
					if !ok {
						return
					}
					logger.Infof("got event %s with data '%+v'", evt.Type, evt.Data)
					evt.Done()
					if evt.Type == event.Exit {
						vu.Events().Global.Unsubscribe(sid)
					}
				case <-vu.Context().Done():
					return
				}
			}
		}()
	})

	if len(rm.vuEvents) > 0 {
		// NOTE: It would be an improvement to only subscribe to events in VUs
		// that will actually run the VU function (VU IDs > 0), and not in the
		// throwaway VUs used for setup/teardown. But since there's no direct
		// access to the VU ID at this point (it would involve getting it from
		// vu.Runtime()), we subscribe in all VUs, and all event handler
		// goroutines would exit normally once rm.exit is closed.
		sid, evtCh := vu.Events().Local.Subscribe(rm.vuEvents...)
		logger := vu.InitEnv().Logger
		rm.WG.Add(1)
		go func() {
			defer rm.WG.Done()
			for {
				select {
				case evt, ok := <-evtCh:
					if !ok {
						return
					}
					logger.Infof("got event %s with data '%+v'", evt.Type, evt.Data)
					evt.Done()
				case <-rm.exit:
					vu.Events().Local.Unsubscribe(sid)
					return
				}
			}
		}()
	}

	return &Events{}
}

// Exports returns the exports of the k6 module.
func (e *Events) Exports() modules.Exports {
	return modules.Exports{Default: e}
}
