// Package redis implements a redis client for k6.
package redis

import (
	"errors"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu modules.VU

		*Client
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New returns a pointer to a new RootModule instance
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu, Client: &Client{vu: vu}}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]interface{}{
		"Client": mi.NewClient,
	}}
}

// NewClient is the JS constructor for the redis Client.
//
// Under the hood, the redis.UniversalClient will be used. The universal client
// supports failover/sentinel, cluster and single-node modes. Depending on the options,
// the internal universal client instance will be one of those.
//
// The type of the underlying client depends on the following conditions:
// If the first argument is a string, it's parsed as a Redis URL, and a
// single-node Client is used.
// Otherwise, an object is expected, and depending on its properties:
// 1. If the masterName property is defined, a sentinel-backed FailoverClient is used.
// 2. If the cluster property is defined, a ClusterClient is used.
// 3. Otherwise, a single-node Client is used.
//
// To support being instantiated in the init context, while not
// producing any IO, as it is the convention in k6, the produced
// Client is initially configured, but in a disconnected state.
// The connection is automatically established when using any of the Redis
// commands exposed by the Client.
func (mi *ModuleInstance) NewClient(call goja.ConstructorCall) *goja.Object {
	rt := mi.vu.Runtime()

	if len(call.Arguments) != 1 {
		common.Throw(rt, errors.New("must specify one argument"))
	}

	opts, err := readOptions(call.Arguments[0].Export())
	if err != nil {
		common.Throw(rt, err)
	}

	client := &Client{
		vu:           mi.vu,
		redisOptions: opts,
		redisClient:  nil,
	}

	return rt.ToValue(client).ToObject(rt)
}
