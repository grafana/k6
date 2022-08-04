package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the execution module.
	ModuleInstance struct {
		vu  modules.VU
		obj *goja.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{vu: vu}
	rt := vu.Runtime()
	o := rt.NewObject()
	defProp := func(name string, newInfo func() (*goja.Object, error)) {
		err := o.DefineAccessorProperty(name, rt.ToValue(func() goja.Value {
			obj, err := newInfo()
			if err != nil {
				common.Throw(rt, err)
			}
			return obj
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, err)
		}
	}
	defProp("instance", mi.newInstanceInfo)
	defProp("scenario", mi.newScenarioInfo)
	defProp("test", mi.newTestInfo)
	defProp("vu", mi.newVUInfo)

	mi.obj = o

	return mi
}

// Exports returns the exports of the execution module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

var errRunInInitContext = errors.New("getting scenario information outside of the VU context is not supported")

// newScenarioInfo returns a goja.Object with property accessors to retrieve
// information about the scenario the current VU is running in.
func (mi *ModuleInstance) newScenarioInfo() (*goja.Object, error) {
	rt := mi.vu.Runtime()
	vuState := mi.vu.State()
	if vuState == nil {
		return nil, errRunInInitContext
	}
	getScenarioState := func() *lib.ScenarioState {
		ss := lib.GetScenarioState(mi.vu.Context())
		if ss == nil {
			common.Throw(rt, errRunInInitContext)
		}
		return ss
	}

	si := map[string]func() interface{}{
		"name": func() interface{} {
			return getScenarioState().Name
		},
		"executor": func() interface{} {
			return getScenarioState().Executor
		},
		"startTime": func() interface{} {
			//nolint:lll
			// Return the timestamp in milliseconds, since that's how JS
			// timestamps usually are:
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/Date#time_value_or_timestamp_number
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/now#return_value
			return getScenarioState().StartTime.UnixNano() / int64(time.Millisecond)
		},
		"progress": func() interface{} {
			p, _ := getScenarioState().ProgressFn()
			return p
		},
		"iterationInInstance": func() interface{} {
			if vuState.GetScenarioLocalVUIter == nil {
				common.Throw(rt, errRunInInitContext)
			}

			return vuState.GetScenarioLocalVUIter()
		},
		"iterationInTest": func() interface{} {
			if vuState.GetScenarioGlobalVUIter == nil {
				common.Throw(rt, errRunInInitContext)
			}

			return vuState.GetScenarioGlobalVUIter()
		},
	}

	return newInfoObj(rt, si)
}

// newInstanceInfo returns a goja.Object with property accessors to retrieve
// information about the local instance stats.
func (mi *ModuleInstance) newInstanceInfo() (*goja.Object, error) {
	es := lib.GetExecutionState(mi.vu.Context())
	if es == nil {
		return nil, errors.New("getting instance information in the init context is not supported")
	}
	rt := mi.vu.Runtime()

	ti := map[string]func() interface{}{
		"currentTestRunDuration": func() interface{} {
			return float64(es.GetCurrentTestRunDuration()) / float64(time.Millisecond)
		},
		"iterationsCompleted": func() interface{} {
			return es.GetFullIterationCount()
		},
		"iterationsInterrupted": func() interface{} {
			return es.GetPartialIterationCount()
		},
		"vusActive": func() interface{} {
			return es.GetCurrentlyActiveVUsCount()
		},
		"vusInitialized": func() interface{} {
			return es.GetInitializedVUsCount()
		},
	}

	return newInfoObj(rt, ti)
}

// newTestInfo returns a goja.Object with property accessors to retrieve
// information and control execution of the overall test run.
func (mi *ModuleInstance) newTestInfo() (*goja.Object, error) {
	// the cache of goja.Object in the optimal parsed form
	// for the consolidated and derived lib.Options
	var optionsObject *goja.Object
	rt := mi.vu.Runtime()
	ti := map[string]func() interface{}{
		// stop the test run
		"abort": func() interface{} {
			return func(msg goja.Value) {
				reason := errext.AbortTest
				if msg != nil && !goja.IsUndefined(msg) {
					reason = fmt.Sprintf("%s: %s", reason, msg.String())
				}
				rt.Interrupt(&errext.InterruptError{Reason: reason})
			}
		},
		"options": func() interface{} {
			if optionsObject == nil {
				opts, err := optionsAsObject(rt, mi.vu.State().Options)
				if err != nil {
					common.Throw(rt, err)
				}
				optionsObject = opts
			}
			return optionsObject
		},
	}

	return newInfoObj(rt, ti)
}

// newVUInfo returns a goja.Object with property accessors to retrieve
// information about the currently executing VU.
func (mi *ModuleInstance) newVUInfo() (*goja.Object, error) {
	vuState := mi.vu.State()
	if vuState == nil {
		return nil, errors.New("getting VU information in the init context is not supported")
	}
	rt := mi.vu.Runtime()

	vi := map[string]func() interface{}{
		"idInInstance":        func() interface{} { return vuState.VUID },
		"idInTest":            func() interface{} { return vuState.VUIDGlobal },
		"iterationInInstance": func() interface{} { return vuState.Iteration },
		"iterationInScenario": func() interface{} {
			return vuState.GetScenarioVUIter()
		},
	}

	o, err := newInfoObj(rt, vi)
	if err != nil {
		return o, err
	}

	err = o.Set("tags", rt.NewDynamicObject(&tagsDynamicObject{
		Runtime: rt,
		State:   vuState,
	}))
	return o, err
}

func newInfoObj(rt *goja.Runtime, props map[string]func() interface{}) (*goja.Object, error) {
	o := rt.NewObject()

	for p, get := range props {
		err := o.DefineAccessorProperty(p, rt.ToValue(get), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// optionsAsObject maps the lib.Options struct that contains the consolidated
// and derived options configuration in a goja.Object.
//
// When values are not set then the default value returned from JSON is used.
// Most of the lib.Options are Nullable types so they will be null on default.
func optionsAsObject(rt *goja.Runtime, options lib.Options) (*goja.Object, error) {
	b, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the lib.Options as json: %w", err)
	}

	// Using the native JS parser function guarantees getting
	// the supported types for deep freezing the complex object.
	jsonParse, _ := goja.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("parse"))
	parsed, err := jsonParse(goja.Undefined(), rt.ToValue(string(b)))
	if err != nil {
		common.Throw(rt, err)
	}

	obj := parsed.ToObject(rt)

	mustDelete := func(prop string) {
		delErr := obj.Delete(prop)
		if err != nil {
			common.Throw(rt, delErr)
		}
	}
	mustSetReadOnlyProperty := func(k string, v interface{}) {
		defErr := obj.DefineDataProperty(k, rt.ToValue(v), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, defErr)
		}
	}

	mustDelete("vus")
	mustDelete("iterations")
	mustDelete("duration")
	mustDelete("stages")

	consoleOutput := goja.Null()
	if options.ConsoleOutput.Valid {
		consoleOutput = rt.ToValue(options.ConsoleOutput.String)
	}
	mustSetReadOnlyProperty("consoleOutput", consoleOutput)

	localIPs := goja.Null()
	if options.LocalIPs.Valid {
		raw, marshalErr := options.LocalIPs.MarshalText()
		if err != nil {
			common.Throw(rt, marshalErr)
		}
		localIPs = rt.ToValue(string(raw))
	}
	mustSetReadOnlyProperty("localIPs", localIPs)

	err = common.FreezeObject(rt, obj)
	if err != nil {
		common.Throw(rt, err)
	}

	return obj, nil
}

type tagsDynamicObject struct {
	Runtime *goja.Runtime
	State   *lib.State
}

// Get a property value for the key. May return nil if the property does not exist.
func (o *tagsDynamicObject) Get(key string) goja.Value {
	tag, ok := o.State.Tags.Get(key)
	if !ok {
		return nil
	}
	return o.Runtime.ToValue(tag)
}

// Set a property value for the key. It returns true if succeed.
// String, Boolean and Number types are implicitly converted
// to the goja's relative string representation.
// In any other case, if the Throw option is set then an error is raised
// otherwise just a Warning is written.
func (o *tagsDynamicObject) Set(key string, val goja.Value) bool {
	kind := reflect.Invalid
	if typ := val.ExportType(); typ != nil {
		kind = typ.Kind()
	}
	switch kind {
	case
		reflect.String,
		reflect.Bool,
		reflect.Int64,
		reflect.Float64:

		o.State.Tags.Set(key, val.String())
		return true
	default:
		reason := "only String, Boolean and Number types are accepted as a Tag value"
		if o.State.Options.Throw.Bool {
			panic(o.Runtime.NewTypeError(reason))
		}
		o.State.Logger.Warnf("the execution.vu.tags.Set('%s') operation has been discarded because %s", key, reason)
		return false
	}
}

// Has returns true if the property exists.
func (o *tagsDynamicObject) Has(key string) bool {
	_, ok := o.State.Tags.Get(key)
	return ok
}

// Delete deletes the property for the key. It returns true on success (note, that includes missing property).
func (o *tagsDynamicObject) Delete(key string) bool {
	o.State.Tags.Delete(key)
	return true
}

// Keys returns a slice with all existing property keys. The order is not deterministic.
func (o *tagsDynamicObject) Keys() []string {
	if o.State.Tags.Len() < 1 {
		return nil
	}

	tags := o.State.Tags.Clone()
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	return keys
}
