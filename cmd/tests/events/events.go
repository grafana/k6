// Package events is used for testing the event functionality.
package events

import (
	"sync"

	"go.k6.io/k6/event"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct {
	initOnce        sync.Once
	subscribeEvents []event.Type
}

// Events represents an instance of the events module.
type Events struct{}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Events{}
)

// New returns a pointer to a new RootModule instance.
func New(subscribeEvents []event.Type) *RootModule {
	return &RootModule{
		initOnce:        sync.Once{},
		subscribeEvents: subscribeEvents,
	}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rm.initOnce.Do(func() {
		sid, evtCh := vu.Events().Subscribe(rm.subscribeEvents...)
		logger := vu.InitEnv().Logger
		go func() {
			for {
				select {
				case evt, ok := <-evtCh:
					if !ok {
						return
					}
					logger.Infof("got event %s with data '%+v'", evt.Type, evt.Data)
					evt.Done()
					if evt.Type == event.Exit {
						vu.Events().Unsubscribe(sid)
					}
				case <-vu.Context().Done():
					return
				}
			}
		}()
	})
	return &Events{}
}

// Exports returns the exports of the k6 module.
func (e *Events) Exports() modules.Exports {
	return modules.Exports{Default: e}
}
