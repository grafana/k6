// Package kontext implements a k6 module that allows users to share values across
// VUs and scenarios.
package kontext

import (
	"fmt"
	"os"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"

	pyroscope "github.com/grafana/pyroscope-go"
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
	// For the Hackathon purpose, we are using Pyroscope to profile the module
	// execution. This is not necessary for the module to work.
	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "k6.hack.kontext",

		// replace this with the address of pyroscope server

		// you can disable logging by setting this to nil
		Logger: nil,

		// by default all profilers are enabled,
		// but you can select the ones you want to use:
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
		},
	})
	if err != nil {
		panic(fmt.Errorf("failed to start pyroscope client: %w", err))
	}

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
	if mi.vu.State() != nil {
		common.Throw(mi.vu.Runtime(), fmt.Errorf("kontext instances can only be created in the init context"))
	}

	serviceURL, hasServiceURL := os.LookupEnv(k6ServiceURLEnvironmentVariable)
	secure := strings.ToLower(os.Getenv(secureEnvironmentVariable)) != "false"

	var kv Kontexter
	var err error
	if hasServiceURL {
		kv, err = NewCloudKontext(mi.vu, serviceURL, secure)
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

	keyStr := key.String()

	go func() {
		value, err := k.kv.Get(keyStr)
		if err != nil {
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

	exportedValue := value.Export()

	go func() {
		err := k.kv.Set(key.String(), exportedValue)
		if err != nil {
			reject(err)
			return
		}

		resolve(nil)
	}()

	return promise
}

// Lpush exposes the operation of pushing a value to the left of a list to the k6 runtime.
func (k *Kontext) Lpush(key sobek.Value, value sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	go func() {
		n, err := k.kv.LeftPush(key.String(), value.Export())
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Rpush exposes the operation of pushing a value to the right of a list to the k6 runtime.
func (k *Kontext) Rpush(key sobek.Value, value sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	go func() {
		n, err := k.kv.RightPush(key.String(), value.Export())
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Lpop exposes the operation of popping a value from the left of a list to the k6 runtime.
func (k *Kontext) Lpop(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	if common.IsNullish(key) {
		reject(fmt.Errorf("key must be a non-empty string"))
		return promise
	}

	// Everything is a reference in JS, so we need to immediately copy the
	// content of the argument before using it in the promise goroutine, to
	// avoid future modifications to the argument affecting the promise (in case a variable
	// is used as the argument, as opposed to a static string).
	keyStr := key.String()

	go func() {
		value, err := k.kv.LeftPop(keyStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Rpop exposes the operation of popping a value from the right of a list to the k6 runtime.
func (k *Kontext) Rpop(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	if common.IsNullish(key) {
		reject(fmt.Errorf("key must be a non-empty string"))
		return promise
	}

	// Everything is a reference in JS, so we need to immediately copy the
	// content of the argument before using it in the promise goroutine, to
	// avoid future modifications to the argument affecting the promise (in case a variable
	// is used as the argument, as opposed to a static string).
	keyStr := key.String()

	go func() {
		value, err := k.kv.RightPop(keyStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Size exposes the operation of getting the size of a list to the k6 runtime.
func (k *Kontext) Size(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	if common.IsNullish(key) {
		reject(fmt.Errorf("key must be a non-empty string"))
		return promise
	}

	// Everything is a reference in JS, so we need to immediately copy the
	// content of the argument before using it in the promise goroutine, to
	// avoid future modifications to the argument affecting the promise (in case a variable
	// is used as the argument, as opposed to a static string).
	keyStr := key.String()

	go func() {
		size, err := k.kv.Size(keyStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(size)
	}()

	return promise
}

// Incr exposes the operation of increasing an integer value by one.
func (k *Kontext) Incr(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	if common.IsNullish(key) {
		reject(fmt.Errorf("key must be a non-empty string"))
		return promise
	}

	// Everything is a reference in JS, so we need to immediately copy the
	// content of the argument before using it in the promise goroutine, to
	// avoid future modifications to the argument affecting the promise (in case a variable
	// is used as the argument, as opposed to a static string).
	keyStr := key.String()

	go func() {
		n, err := k.kv.Incr(keyStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Decr exposes the operation of decreasing an integer value by one.
func (k *Kontext) Decr(key sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(k.vu)

	if common.IsNullish(key) {
		reject(fmt.Errorf("key must be a non-empty string"))
		return promise
	}

	// Everything is a reference in JS, so we need to immediately copy the
	// content of the argument before using it in the promise goroutine, to
	// avoid future modifications to the argument affecting the promise (in case a variable
	// is used as the argument, as opposed to a static string).
	keyStr := key.String()

	go func() {
		n, err := k.kv.Decr(keyStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}
