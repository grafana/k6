package sse

import (
	"go.k6.io/k6-extension-api"
	"go.k6.io/k6-extension-api/common"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}
)

var (
	_ extensionapi.Module   = &RootModule{}
	_ extensionapi.Instance = &sse{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m extensionapi.VU) extensionapi.Instance {
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
	extensionapi.Register("k6/x/sse", new(RootModule))
}
