// Package execution implements k6/execution which lets script find out more about it is execution.
package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the execution module.
	ModuleInstance struct {
		vu  modules.VU
		obj *sobek.Object
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
	defProp := func(name string, newInfo func() (*sobek.Object, error)) {
		err := o.DefineAccessorProperty(name, rt.ToValue(func() sobek.Value {
			obj, err := newInfo()
			if err != nil {
				common.Throw(rt, err)
			}
			return obj
		}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
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

// newScenarioInfo returns a sobek.Object with property accessors to retrieve
// information about the scenario the current VU is running in.
func (mi *ModuleInstance) newScenarioInfo() (*sobek.Object, error) {
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

//nolint:lll
var errInstanceInfoInitContext = common.NewInitContextError("getting instance information in the init context is not supported")

// newInstanceInfo returns a sobek.Object with property accessors to retrieve
// information about the local instance stats.
func (mi *ModuleInstance) newInstanceInfo() (*sobek.Object, error) {
	es := lib.GetExecutionState(mi.vu.Context())
	if es == nil {
		return nil, errInstanceInfoInitContext
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

var errTestInfoInitContext = common.NewInitContextError("getting test options in the init context is not supported")

// newTestInfo returns a sobek.Object with property accessors to retrieve
// information and control execution of the overall test run.
func (mi *ModuleInstance) newTestInfo() (*sobek.Object, error) {
	// the cache of sobek.Object in the optimal parsed form
	// for the consolidated and derived lib.Options
	var optionsObject *sobek.Object
	rt := mi.vu.Runtime()
	ti := map[string]func() interface{}{
		// stop the test run
		"abort": func() interface{} {
			return func(msg sobek.Value) {
				reason := errext.AbortTest
				if msg != nil && !sobek.IsUndefined(msg) {
					reason = fmt.Sprintf("%s: %s", reason, msg.String())
				}
				rt.Interrupt(&errext.InterruptError{Reason: reason})
			}
		},
		"options": func() interface{} {
			vuState := mi.vu.State()
			if vuState == nil {
				common.Throw(rt, errTestInfoInitContext)
			}
			if optionsObject == nil {
				opts, err := optionsAsObject(rt, vuState.Options)
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

var errVUInfoInitContex = common.NewInitContextError("getting VU information in the init context is not supported")

// newVUInfo returns a sobek.Object with property accessors to retrieve
// information about the currently executing VU.
func (mi *ModuleInstance) newVUInfo() (*sobek.Object, error) {
	vuState := mi.vu.State()
	if vuState == nil {
		return nil, errVUInfoInitContex
	}
	rt := mi.vu.Runtime()

	vi := map[string]func() interface{}{
		"idInInstance":        func() interface{} { return vuState.VUID },
		"idInTest":            func() interface{} { return vuState.VUIDGlobal },
		"iterationInInstance": func() interface{} { return vuState.Iteration },
		"iterationInScenario": func() interface{} {
			if vuState.GetScenarioVUIter == nil {
				// hasn't been set yet, no iteration stats available
				return 0
			}

			return vuState.GetScenarioVUIter()
		},
	}

	o, err := newInfoObj(rt, vi)
	if err != nil {
		return o, err
	}
	tagsDynamicObject := rt.NewDynamicObject(&tagsDynamicObject{
		runtime: rt,
		state:   vuState,
	})

	// This is kept for backwards compatibility reasons, but should be deprecated,
	// since tags are also accessible via vu.metrics.tags.
	err = o.Set("tags", tagsDynamicObject)
	if err != nil {
		return o, err
	}
	metrics, err := newInfoObj(rt, map[string]func() interface{}{
		"tags": func() interface{} { return tagsDynamicObject },
		"metadata": func() interface{} {
			return rt.NewDynamicObject(&metadataDynamicObject{
				runtime: rt,
				state:   vuState,
			})
		},
	})
	if err != nil {
		return o, err
	}

	err = o.Set("metrics", metrics)
	if err != nil {
		return o, err
	}

	return o, err
}

func newInfoObj(rt *sobek.Runtime, props map[string]func() interface{}) (*sobek.Object, error) {
	o := rt.NewObject()

	for p, get := range props {
		err := o.DefineAccessorProperty(p, rt.ToValue(get), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// optionsAsObject maps the lib.Options struct that contains the consolidated
// and derived options configuration in a sobek.Object.
//
// When values are not set then the default value returned from JSON is used.
// Most of the lib.Options are Nullable types so they will be null on default.
func optionsAsObject(rt *sobek.Runtime, options lib.Options) (*sobek.Object, error) {
	b, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the lib.Options as json: %w", err)
	}

	// Using the native JS parser function guarantees getting
	// the supported types for deep freezing the complex object.
	jsonParse, _ := sobek.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("parse"))
	parsed, err := jsonParse(sobek.Undefined(), rt.ToValue(string(b)))
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
		defErr := obj.DefineDataProperty(k, rt.ToValue(v), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, defErr)
		}
	}

	mustDelete("vus")
	mustDelete("iterations")
	mustDelete("duration")
	mustDelete("stages")

	consoleOutput := sobek.Null()
	if options.ConsoleOutput.Valid {
		consoleOutput = rt.ToValue(options.ConsoleOutput.String)
	}
	mustSetReadOnlyProperty("consoleOutput", consoleOutput)

	localIPs := sobek.Null()
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
	runtime *sobek.Runtime
	state   *lib.State
}

// Get a property value for the key. May return nil if the property does not exist.
func (o *tagsDynamicObject) Get(key string) sobek.Value {
	tcv := o.state.Tags.GetCurrentValues()
	if tag, ok := tcv.Tags.Get(key); ok {
		return o.runtime.ToValue(tag)
	}
	return nil
}

// Set a property value for the key. It returns true if succeed. String, Boolean
// and Number types are implicitly converted to the Sobek's relative string
// representation. An exception is raised in case a denied type is provided.
func (o *tagsDynamicObject) Set(key string, val sobek.Value) bool {
	o.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		if err := common.ApplyCustomUserTag(tagsAndMeta, key, val); err != nil {
			panic(o.runtime.NewTypeError(err.Error()))
		}
	})
	return true
}

// Has returns true if the property exists.
func (o *tagsDynamicObject) Has(key string) bool {
	ctv := o.state.Tags.GetCurrentValues()
	if _, ok := ctv.Tags.Get(key); ok {
		return true
	}
	return false
}

// Delete deletes the property for the key. It returns true on success (note,
// that includes missing property).
func (o *tagsDynamicObject) Delete(key string) bool {
	o.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		tagsAndMeta.DeleteTag(key)
	})
	return true
}

// Keys returns a slice with all existing property keys. The order is not
// deterministic.
func (o *tagsDynamicObject) Keys() []string {
	ctv := o.state.Tags.GetCurrentValues()

	tagsMap := ctv.Tags.Map()
	keys := make([]string, 0, len(tagsMap)+len(ctv.Metadata))
	for k := range tagsMap {
		keys = append(keys, k)
	}
	return keys
}

type metadataDynamicObject struct {
	runtime *sobek.Runtime
	state   *lib.State
}

// Get a property value for the key. May return nil if the property does not exist.
func (o *metadataDynamicObject) Get(key string) sobek.Value {
	tcv := o.state.Tags.GetCurrentValues()
	if metadatum, ok := tcv.Metadata[key]; ok {
		return o.runtime.ToValue(metadatum)
	}
	return nil
}

// Set a property value for the key. It returns true if successful. String, Boolean
// and Number types are implicitly converted to the Sobek's relative string
// representation. An exception is raised in case a denied type is provided.
func (o *metadataDynamicObject) Set(key string, val sobek.Value) bool {
	o.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		if err := common.ApplyCustomUserMetadata(tagsAndMeta, key, val); err != nil {
			panic(o.runtime.NewTypeError(err.Error()))
		}
	})
	return true
}

// Has returns true if the property exists.
func (o *metadataDynamicObject) Has(key string) bool {
	ctv := o.state.Tags.GetCurrentValues()
	if _, ok := ctv.Metadata[key]; ok {
		return true
	}
	return false
}

// Delete deletes the property for the key. It returns true on success (note,
// that includes missing property).
func (o *metadataDynamicObject) Delete(key string) bool {
	o.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		tagsAndMeta.DeleteMetadata(key)
	})
	return true
}

// Keys returns a slice with all existing property keys. The order is not
// deterministic.
func (o *metadataDynamicObject) Keys() []string {
	ctv := o.state.Tags.GetCurrentValues()

	keys := make([]string, 0, len(ctv.Metadata))
	for k := range ctv.Metadata {
		keys = append(keys, k)
	}
	return keys
}
