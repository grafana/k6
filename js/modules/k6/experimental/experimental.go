// Package experimental includes experimental module features
package experimental

import (
	"errors"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the root experimental module
	RootModule struct{}
	// ModuleInstance represents an instance of the experimental module
	ModuleInstance struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// NewModuleInstance implements modules.Module interface
func (*RootModule) NewModuleInstance(m modules.VU) modules.Instance {
	return &ModuleInstance{vu: m}
}

// New returns a new RootModule.
func New() *RootModule {
	return &RootModule{}
}

// Exports returns the exports of the experimental module
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"setTimeout": mi.setTimeout,
		},
	}
}

func (mi *ModuleInstance) setTimeout(f sobek.Callable, t float64) {
	if f == nil {
		common.Throw(mi.vu.Runtime(), errors.New("setTimeout requires a function as first argument"))
	}
	// TODO maybe really return something to use with `clearTimeout
	// TODO support arguments ... maybe
	runOnLoop := mi.vu.RegisterCallback()
	go func() {
		timer := time.NewTimer(time.Duration(t * float64(time.Millisecond)))
		select {
		case <-timer.C:
			runOnLoop(func() error {
				_, err := f(sobek.Undefined())
				return err
			})
		case <-mi.vu.Context().Done():
			// TODO log something?

			timer.Stop()
			runOnLoop(func() error { return nil })
		}
	}()
}
