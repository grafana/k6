package sql

import (
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modules"
)

type registry struct {
	mu      sync.RWMutex
	drivers map[*sobek.Symbol]struct{}
}

var driverRegistry *registry //nolint:gochecknoglobals

func init() {
	driverRegistry = &registry{
		drivers: make(map[*sobek.Symbol]struct{}),
	}
}

func (r *registry) register(driverName string) *sobek.Symbol {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := sobek.NewSymbol(driverName)

	r.drivers[id] = struct{}{}

	return id
}

func (r *registry) lookup(id *sobek.Symbol) (bool, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, found := r.drivers[id]
	if !found {
		return false, ""
	}

	return true, id.String()
}

// RegisterDriver registers an SQL database driver.
func RegisterDriver(driverName string) *sobek.Symbol {
	return driverRegistry.register(driverName)
}

// driverImportPathPrefix contains driver module's common JavaScript import path prefix.
const driverImportPathPrefix = "k6/x/sql/driver/"

// RegisterModule registers an SQL database driver module.
// The module import path will be k6/x/sql/driver/ + driverName.
// The module's default export will be the driver id Symbol.
func RegisterModule(driverName string) modules.Module {
	id := RegisterDriver(driverName)
	root := &driverRootModule{driverID: id}

	modules.Register(driverImportPathPrefix+driverName, root)

	return root
}

// lookupDriver search SQL database driver by id in the registry.
// Returns true and the driver name if the driver is registered,
// otherwise returns false and an empty string.
func lookupDriver(id *sobek.Symbol) (bool, string) {
	return driverRegistry.lookup(id)
}

// driverRootModule is the global module object type. It is instantiated once per test
// run and will be used to create `k6/x/sql/driver/XXX` module instances for each VU.
type driverRootModule struct {
	driverID *sobek.Symbol
}

var _ modules.Module = &driverRootModule{}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (root *driverRootModule) NewModuleInstance(_ modules.VU) modules.Instance {
	instance := &driverModule{
		exports: modules.Exports{
			Default: root.driverID,
		},
	}

	return instance
}

// module represents an instance of the JavaScript module for every VU.
type driverModule struct {
	exports modules.Exports
}

// Exports is representation of ESM exports of a module.
func (mod *driverModule) Exports() modules.Exports {
	return mod.exports
}
