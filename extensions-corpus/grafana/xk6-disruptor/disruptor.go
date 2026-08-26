// Package disruptor implement the k6 extension interface for calling disruptors from js scripts
// running in the goya runtime
package disruptor

import (
	"fmt"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-disruptor/pkg/api"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
)

func init() {
	modules.Register("k6/x/disruptor", new(RootModule))
}

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create `k6/x/disruptor` module instances for each VU.
type RootModule struct{}

// ModuleInstance represents an instance of the JS module.
type ModuleInstance struct {
	vu modules.VU
	// instance of a Kubernetes helper
	k8s kubernetes.Kubernetes
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// NewModuleInstance returns a new instance of the disruptor module for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	k8s, err := kubernetes.New()
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("error creating Kubernetes helper: %w", err))
	}

	return &ModuleInstance{
		vu:  vu,
		k8s: k8s,
	}
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (m *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"PodDisruptor":     m.newPodDisruptor,
			"ServiceDisruptor": m.newServiceDisruptor,
		},
	}
}

// creates an instance of a PodDisruptor
func (m *ModuleInstance) newPodDisruptor(c sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()
	ctx := m.vu.Context()

	disruptor, err := api.NewPodDisruptor(ctx, rt, c, m.k8s)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating PodDisruptor: %w", err))
	}
	return disruptor
}

// creates an instance of a ServiceDisruptor
func (m *ModuleInstance) newServiceDisruptor(c sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()
	ctx := m.vu.Context()

	disruptor, err := api.NewServiceDisruptor(ctx, rt, c, m.k8s)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating ServiceDisruptor: %w", err))
	}

	return disruptor
}
