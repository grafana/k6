// Package kontext implements a k6 module that allows users to share values across
// VUs and scenarios.
package kontext

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"
)

type (
	// RootModule is the global module instance that will create instances of our
	// module for each VU.
	RootModule struct {
		db *db
	}

	// ModuleInstance represents an instance of the fs module for a single VU.
	ModuleInstance struct {
		vu modules.VU

		rm *RootModule
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new [RootModule] instance.
func New() *RootModule {
	return &RootModule{db: newDB()}
}

// NewModuleInstance implements the modules.Module interface and returns a new
// instance of our module for the given VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu, rm: rm}
}

// Exports implements the modules.Module interface and returns the exports of
// our module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Kontext": mi.NewKontext,
		},
	}
}

// NewKontext creates a new Kontext object.
func (mi *ModuleInstance) NewKontext(_ sobek.ConstructorCall) *sobek.Object {
	serviceURL, hasServiceURL := os.LookupEnv(k6ServiceURLEnvironmentVariable)

	var kv Kontexter
	var err error
	if hasServiceURL {
		kv, err = NewCloudKontext(mi.vu, serviceURL)
		if err != nil {
			common.Throw(mi.vu.Runtime(), fmt.Errorf("failed to create new Kontext instance: %w", err))
		}
	} else {
		kv, err = NewLocalKontext(mi.vu, mi.rm.db)
		if err != nil {
			common.Throw(mi.vu.Runtime(), fmt.Errorf("failed to create new Kontext instance: %w", err))
		}
	}

	k := &Kontext{
		vu: mi.vu,
		kv: kv,
	}

	return mi.vu.Runtime().ToValue(k).ToObject(mi.vu.Runtime())
}

// Kontext represents a shared context that can be used to share values across
// VUs and scenarios.
type Kontext struct {
	vu modules.VU

	kv Kontexter
}

// Get retrieves a value from the shared context.
func (k *Kontext) Get(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	go func() {
		jsonValue, err := k.kv.Get(key.String())
		if err != nil {
			reject(err)
			return
		}

		if jsonValue == nil {
			reject(KontextKeyNotFoundError)
			return
		}

		var value any
		if err := json.Unmarshal(jsonValue, &value); err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Set sets a value in the shared context.
func (k *Kontext) Set(key sobek.Value, value sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	jsonValue, err := json.Marshal(value.Export())
	if err != nil {
		reject(fmt.Errorf("failed to marshal value to json"))
		return promise
	}

	go func() {
		err := k.kv.Set(key.String(), jsonValue)
		if err != nil {
			reject(err)
			return
		}

		resolve(nil)
	}()

	return promise
}
