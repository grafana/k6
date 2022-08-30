package goja

import (
	"errors"
)

type HostResolveImportedModuleFunc func(referencingScriptOrModule interface{}, specifier string) (ModuleRecord, error)

// TODO most things here probably should be unexported and names should be revised before merged in master
// Record should probably be dropped from everywhere

// ModuleRecord is the common interface for module record as defined in the EcmaScript specification
type ModuleRecord interface {
	GetExportedNames(resolveset ...ModuleRecord) []string
	ResolveExport(exportName string, resolveset ...ResolveSetElement) (*ResolvedBinding, bool)
	Link() error
	Evaluate(*Runtime) *Promise
}

type CyclicModuleRecordStatus uint8

const (
	Unlinked CyclicModuleRecordStatus = iota
	Linking
	Linked
	Evaluating
	Evaluating_Async
	Evaluated
)

type CyclicModuleRecord interface {
	ModuleRecord
	RequestedModules() []string
	InitializeEnvironment() error
	Instantiate(rt *Runtime) (CyclicModuleInstance, error)
}

type (
	ModuleInstance interface {
		GetBindingValue(string) Value
	}
	CyclicModuleInstance interface {
		ModuleInstance
		HasTLA() bool
		ExecuteModule(rt *Runtime, res, rej func(interface{})) (CyclicModuleInstance, error)
	}
)

type linkState struct {
	status           map[ModuleRecord]CyclicModuleRecordStatus
	dfsIndex         map[ModuleRecord]uint
	dfsAncestorIndex map[ModuleRecord]uint
}

func newLinkState() *linkState {
	return &linkState{
		status:           make(map[ModuleRecord]CyclicModuleRecordStatus),
		dfsIndex:         make(map[ModuleRecord]uint),
		dfsAncestorIndex: make(map[ModuleRecord]uint),
	}
}

func (c *compiler) CyclicModuleRecordConcreteLink(module ModuleRecord) error {
	stack := []CyclicModuleRecord{}
	if _, err := c.innerModuleLinking(newLinkState(), module, &stack, 0); err != nil {
		return err
	}
	return nil
}

func (c *compiler) innerModuleLinking(state *linkState, m ModuleRecord, stack *[]CyclicModuleRecord, index uint) (uint, error) {
	var module CyclicModuleRecord
	var ok bool
	if module, ok = m.(CyclicModuleRecord); !ok {
		return index, m.Link()
	}
	if status := state.status[module]; status == Linking || status == Linked || status == Evaluated {
		return index, nil
	} else if status != Unlinked {
		return 0, errors.New("bad status on link") // TODO fix
	}
	state.status[module] = Linking
	state.dfsIndex[module] = index
	state.dfsAncestorIndex[module] = index
	index++
	*stack = append(*stack, module)
	var err error
	var requiredModule ModuleRecord
	for _, required := range module.RequestedModules() {
		requiredModule, err = c.hostResolveImportedModule(module, required)
		if err != nil {
			return 0, err
		}
		index, err = c.innerModuleLinking(state, requiredModule, stack, index)
		if err != nil {
			return 0, err
		}
		if requiredC, ok := requiredModule.(CyclicModuleRecord); ok {
			if state.status[requiredC] == Linking {
				if ancestorIndex := state.dfsAncestorIndex[module]; state.dfsAncestorIndex[requiredC] > ancestorIndex {
					state.dfsAncestorIndex[requiredC] = ancestorIndex
				}
			}
		}
	}
	err = module.InitializeEnvironment()
	if err != nil {
		return 0, err
	}
	if state.dfsAncestorIndex[module] == state.dfsIndex[module] {
		for i := len(*stack) - 1; i >= 0; i-- {
			requiredModule := (*stack)[i]
			*stack = (*stack)[:i]
			state.status[requiredModule] = Linked
			if requiredModule == module {
				break
			}
		}
	}
	return index, nil
}

type evaluationState struct {
	status                   map[ModuleInstance]CyclicModuleRecordStatus
	dfsIndex                 map[ModuleInstance]uint
	dfsAncestorIndex         map[ModuleInstance]uint
	pendingAsyncDependancies map[ModuleInstance]uint
	cycleRoot                map[ModuleInstance]CyclicModuleInstance
	asyncEvaluation          map[CyclicModuleInstance]bool
	asyncParentModules       map[CyclicModuleInstance][]CyclicModuleInstance
	evaluationError          map[CyclicModuleInstance]error
	topLevelCapability       map[CyclicModuleRecord]*promiseCapability
}

func newEvaluationState() *evaluationState {
	return &evaluationState{
		status:                   make(map[ModuleInstance]CyclicModuleRecordStatus),
		dfsIndex:                 make(map[ModuleInstance]uint),
		dfsAncestorIndex:         make(map[ModuleInstance]uint),
		pendingAsyncDependancies: make(map[ModuleInstance]uint),
		cycleRoot:                make(map[ModuleInstance]CyclicModuleInstance),
		asyncEvaluation:          make(map[CyclicModuleInstance]bool),
		asyncParentModules:       make(map[CyclicModuleInstance][]CyclicModuleInstance),
		evaluationError:          make(map[CyclicModuleInstance]error),
		topLevelCapability:       make(map[CyclicModuleRecord]*promiseCapability),
	}
}

// TODO have resolve as part of runtime
func (r *Runtime) CyclicModuleRecordEvaluate(c CyclicModuleRecord, resolve HostResolveImportedModuleFunc,
) *Promise {
	if r.modules == nil {
		r.modules = make(map[ModuleRecord]ModuleInstance)
	}
	// TODO implement all the promise stuff
	stackInstance := []CyclicModuleInstance{}

	state := newEvaluationState()
	capability := r.newPromiseCapability(r.global.Promise)
	state.topLevelCapability[c] = capability
	// TODO fix abrupt result
	_, err := r.innerModuleEvaluation(state, c, &stackInstance, 0, resolve)
	if err != nil {
		for _, m := range stackInstance {
			state.status[m] = Evaluated
			state.evaluationError[m] = err
		}
		capability.reject(r.ToValue(err))

	} else {
		if !state.asyncEvaluation[r.modules[c].(CyclicModuleInstance)] {
			state.topLevelCapability[c].resolve(_undefined)
		}
	}
	// TODO handle completion
	return state.topLevelCapability[c].promise.Export().(*Promise)
}

func (r *Runtime) innerModuleEvaluation(
	state *evaluationState,
	m ModuleRecord, stack *[]CyclicModuleInstance, index uint,
	resolve HostResolveImportedModuleFunc,
) (idx uint, err error) {
	if len(*stack) > 100000 {
		panic("too deep dependancy stack of 100000")
	}
	var cr CyclicModuleRecord
	var ok bool
	var c CyclicModuleInstance
	if cr, ok = m.(CyclicModuleRecord); !ok {
		p := m.Evaluate(r)
		if p.state == PromiseStateRejected {
			return index, p.Result().Export().(error)
		}
		r.modules[m] = p.Result().Export().(ModuleInstance) // TODO fix this cast ... somehow
		return index, nil
	}
	if _, ok = r.modules[m]; ok {
		return index, nil
	}
	c, err = cr.Instantiate(r)
	if err != nil {
		// state.evaluationError[cr] = err
		// TODO handle this somehow - maybe just panic
		return index, err
	}

	r.modules[m] = c
	if status := state.status[c]; status == Evaluated {
		return index, nil
	} else if status == Evaluating || status == Evaluating_Async {
		// maybe check evaluation error
		return index, nil
	}
	state.status[c] = Evaluating
	state.dfsIndex[c] = index
	state.dfsAncestorIndex[c] = index
	state.pendingAsyncDependancies[c] = 0
	index++

	*stack = append(*stack, c)
	var requiredModule ModuleRecord
	for _, required := range cr.RequestedModules() {
		requiredModule, err = resolve(m, required)
		if err != nil {
			state.evaluationError[c] = err
			return index, err
		}
		var requiredInstance ModuleInstance
		index, err = r.innerModuleEvaluation(state, requiredModule, stack, index, resolve)
		if err != nil {
			return index, err
		}
		if requiredC, ok := requiredInstance.(CyclicModuleInstance); ok {
			if state.status[requiredC] == Evaluating {
				if ancestorIndex := state.dfsAncestorIndex[c]; state.dfsAncestorIndex[requiredC] > ancestorIndex {
					state.dfsAncestorIndex[requiredC] = ancestorIndex
				}
			} else {
				requiredC = state.cycleRoot[requiredC]
				// check stuff
			}
			if state.asyncEvaluation[requiredC] {
				state.pendingAsyncDependancies[c]++
				state.asyncParentModules[requiredC] = append(state.asyncParentModules[requiredC], c)
			}
		}
	}
	if state.pendingAsyncDependancies[c] > 0 || c.HasTLA() {
		state.asyncEvaluation[c] = true
		if state.pendingAsyncDependancies[c] == 0 {
			r.executeAsyncModule(state, c)
		}
	} else {
		c, err = c.ExecuteModule(r, nil, nil)
		if err != nil {
			// state.evaluationError[c] = err
			return index, err
		}
	}

	if state.dfsAncestorIndex[c] == state.dfsIndex[c] {
		for i := len(*stack) - 1; i >= 0; i-- {
			requiredModuleInstance := (*stack)[i]
			*stack = (*stack)[:i]
			if !state.asyncEvaluation[requiredModuleInstance] {
				state.status[requiredModuleInstance] = Evaluated
			} else {
				state.status[requiredModuleInstance] = Evaluating_Async
			}
			state.cycleRoot[requiredModuleInstance] = c
			if requiredModuleInstance == c {
				break
			}
		}
	}
	return index, nil
}

func (r *Runtime) executeAsyncModule(state *evaluationState, c CyclicModuleInstance) {
	// implement https://262.ecma-international.org/13.0/#sec-execute-async-module
	// TODO likely wrong
	p, res, rej := r.NewPromise()
	r.performPromiseThen(p, r.ToValue(func() {
		r.asyncModuleExecutionFulfilled(state, c)
	}), r.ToValue(func(err error) {
		r.asyncModuleExecutionRejected(state, c, err)
	}), nil)
	c.ExecuteModule(r, res, rej)
}

func (r *Runtime) asyncModuleExecutionFulfilled(state *evaluationState, c CyclicModuleInstance) {
	if state.status[c] == Evaluated {
		return
	}
	state.asyncEvaluation[c] = false
	// TODO fix this
	for m, i := range r.modules {
		if i == c {
			if cap := state.topLevelCapability[m.(CyclicModuleRecord)]; cap != nil {
				cap.resolve(_undefined)
			}
			break
		}
	}
	execList := make([]CyclicModuleInstance, 0)
	r.gatherAvailableAncestors(state, c, &execList)
	// TODO sort? per when the modules got their AsyncEvaluation set ... somehow
	for _, m := range execList {
		if state.status[m] == Evaluated {
			continue
		}
		if m.HasTLA() {
			r.executeAsyncModule(state, m)
		} else {
			result, err := m.ExecuteModule(r, nil, nil)
			if err != nil {
				r.asyncModuleExecutionRejected(state, m, err)
				continue
			}
			state.status[m] = Evaluated
			if cap := state.topLevelCapability[r.findModuleRecord(c).(CyclicModuleRecord)]; cap != nil {
				// TODO having the module instances going through Values and back is likely not a *great* idea
				cap.resolve(r.ToValue(result))
			}
		}
	}
}

func (r *Runtime) gatherAvailableAncestors(state *evaluationState, c CyclicModuleInstance, execList *[]CyclicModuleInstance) {
	contains := func(m CyclicModuleInstance) bool {
		for _, l := range *execList {
			if l == m {
				return true
			}
		}
		return false
	}
	for _, m := range state.asyncParentModules[c] {
		if contains(m) || state.evaluationError[m] != nil {
			continue
		}
		state.pendingAsyncDependancies[m]--
		if state.pendingAsyncDependancies[m] == 0 {
			*execList = append(*execList, m)
			if !m.HasTLA() {
				r.gatherAvailableAncestors(state, m, execList)
			}
		}
	}
}

func (r *Runtime) asyncModuleExecutionRejected(state *evaluationState, c CyclicModuleInstance, err error) {
	if state.status[c] == Evaluated {
		return
	}
	state.evaluationError[c] = err
	state.status[c] = Evaluated
	for _, m := range state.asyncParentModules[c] {
		r.asyncModuleExecutionRejected(state, m, err)
	}
	// TODO handle top level capabiltiy better
	if cap := state.topLevelCapability[r.findModuleRecord(c).(CyclicModuleRecord)]; cap != nil {
		cap.reject(r.ToValue(err))
	}
}

// TODO fix this whole thing
func (r *Runtime) findModuleRecord(i ModuleInstance) ModuleRecord {
	for m, mi := range r.modules {
		if mi == i {
			return m
		}
	}
	panic("this should never happen")
}

func (r *Runtime) GetActiveScriptOrModule() interface{} { // have some better type
	if r.vm.prg != nil && r.vm.prg.scriptOrModule != nil {
		return r.vm.prg.scriptOrModule
	}
	for i := len(r.vm.callStack) - 1; i >= 0; i-- {
		prg := r.vm.callStack[i].prg
		if prg.scriptOrModule != nil {
			return prg.scriptOrModule
		}
	}
	return nil
}

func (r *Runtime) getImportMetaFor(m ModuleRecord) *Object {
	if r.importMetas == nil {
		r.importMetas = make(map[ModuleRecord]*Object)
	}
	if o, ok := r.importMetas[m]; ok {
		return o
	}
	o := r.NewObject()
	o.SetPrototype(nil)

	var properties []MetaProperty
	if r.getImportMetaProperties != nil {
		properties = r.getImportMetaProperties(m)
	}

	for _, property := range properties {
		o.Set(property.Key, property.Value)
	}

	if r.finalizeImportMeta != nil {
		r.finalizeImportMeta(o, m)
	}

	r.importMetas[m] = o
	return o
}

type MetaProperty struct {
	Key   string
	Value Value
}

func (r *Runtime) SetGetImportMetaProperties(fn func(ModuleRecord) []MetaProperty) {
	r.getImportMetaProperties = fn
}

func (r *Runtime) SetFinalImportMeta(fn func(*Object, ModuleRecord)) {
	r.finalizeImportMeta = fn
}

// TODO fix signature
type ImportModuleDynamicallyCallback func(referencingScriptOrModule interface{}, specifier Value, promiseCapability interface{})

func (r *Runtime) SetImportModuleDynamically(callback ImportModuleDynamicallyCallback) {
	r.importModuleDynamically = callback
}

// TODO figure out the arguments
func (r *Runtime) FinalizeDynamicImport(m ModuleRecord, pcap interface{}, err interface{}) {
	p := pcap.(*promiseCapability)
	if err != nil {
		switch x1 := err.(type) {
		case *Exception:
			p.reject(x1.val)
		case *CompilerSyntaxError:
			p.reject(r.builtin_new(r.global.SyntaxError, []Value{newStringValue(x1.Error())}))
		case *CompilerReferenceError:
			p.reject(r.newError(r.global.ReferenceError, x1.Message))
		default:
			p.reject(r.ToValue(err))
		}
		return
	}
	p.resolve(r.NamespaceObjectFor(m))
}
