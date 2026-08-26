package sse

import (
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &sse{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m modules.VU) modules.Instance {
	rt := m.Runtime()
	mi := &sse{
		vu: m,
	}

	obj := rt.NewObject()
	if err := obj.Set("open", mi.Open); err != nil {
		common.Throw(rt, err)
	}

	mi.obj = obj

	metrics, err := registerMetrics(m)
	if err != nil {
		common.Throw(rt, err)
	}
	mi.metrics = &metrics

	return mi
}

func init() {
	modules.Register("k6/x/sse", new(RootModule))
}
