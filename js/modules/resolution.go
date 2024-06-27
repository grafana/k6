package modules

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/loader"
)

const notPreviouslyResolvedModule = "the module %q was not previously resolved during initialization (__VU==0)"

// FileLoader is a type alias for a function that returns the contents of the referenced file.
type FileLoader func(specifier *url.URL, name string) ([]byte, error)

type moduleCacheElement struct {
	mod sobek.ModuleRecord
	err error
}

// ModuleResolver knows how to get base Module that can be initialized
type ModuleResolver struct {
	cache     map[string]moduleCacheElement
	goModules map[string]any
	loadCJS   FileLoader
	compiler  *compiler.Compiler
	locked    bool
	reverse   map[any]*url.URL // maybe use sobek.ModuleRecord as key
	base      *url.URL
}

// NewModuleResolver returns a new module resolution instance that will resolve.
// goModules is map of import file to a go module
// loadCJS is used to load commonjs files
func NewModuleResolver(
	goModules map[string]any, loadCJS FileLoader, c *compiler.Compiler, base *url.URL,
) *ModuleResolver {
	return &ModuleResolver{
		goModules: goModules,
		cache:     make(map[string]moduleCacheElement),
		loadCJS:   loadCJS,
		compiler:  c,
		reverse:   make(map[any]*url.URL),
		base:      base,
	}
}

func (mr *ModuleResolver) resolveSpecifier(basePWD *url.URL, arg string) (*url.URL, error) {
	specifier, err := loader.Resolve(basePWD, arg)
	if err != nil {
		return nil, err
	}
	return specifier, nil
}

func (mr *ModuleResolver) requireModule(name string) (sobek.ModuleRecord, error) {
	if mr.locked {
		return nil, fmt.Errorf(notPreviouslyResolvedModule, name)
	}
	mod, ok := mr.goModules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	k6m, ok := mod.(Module)
	if !ok {
		return &baseGoModule{m: mod}, nil
	}
	return &goModule{m: k6m}, nil
}

func (mr *ModuleResolver) resolveLoaded(basePWD *url.URL, arg string, data []byte) (sobek.ModuleRecord, error) {
	specifier, err := mr.resolveSpecifier(basePWD, arg)
	if err != nil {
		return nil, err
	}
	// try cache with the final specifier
	if cached, ok := mr.cache[specifier.String()]; ok {
		return cached.mod, cached.err
	}
	prg, isESM, err := mr.compiler.Parse(string(data), specifier.String(), false)
	if err != nil {
		mr.cache[specifier.String()] = moduleCacheElement{err: err}
		return nil, err
	}
	var mod sobek.ModuleRecord
	if isESM {
		mod, err = sobek.ModuleFromAST(prg, mr.sobekModuleResolver)
	} else {
		mod, err = cjsModuleFromString(specifier, data, mr.compiler)
	}
	mr.reverse[mod] = specifier
	mr.cache[specifier.String()] = moduleCacheElement{mod: mod, err: err}
	return mod, err
}

// Lock locks the module's resolution from any further new resolving operation.
// It means that it relays only its internal cache and on the fact that it has already
// seen previously the module during the initialization.
// It is the same approach used for opening file operations.
func (mr *ModuleResolver) Lock() {
	mr.locked = true
}

// fix
type vubox struct {
	vu VU
}

func (mr *ModuleResolver) resolve(basePWD *url.URL, arg string) (sobek.ModuleRecord, error) {
	switch {
	case arg == "k6", strings.HasPrefix(arg, "k6/"):
		// Builtin or external modules ("k6", "k6/*", or "k6/x/*") are handled
		// specially, as they don't exist on the filesystem.
		if cached, ok := mr.cache[arg]; ok {
			return cached.mod, cached.err
		}
		mod, err := mr.requireModule(arg)
		mr.cache[arg] = moduleCacheElement{mod: mod, err: err}
		return mod, err
	default:
		specifier, err := mr.resolveSpecifier(basePWD, arg)
		if err != nil {
			return nil, err
		}
		// try cache with the final specifier
		if cached, ok := mr.cache[specifier.String()]; ok {
			return cached.mod, cached.err
		}

		if mr.locked {
			return nil, fmt.Errorf(notPreviouslyResolvedModule, arg)
		}
		// Fall back to loading
		data, err := mr.loadCJS(specifier, arg)
		if err != nil {
			mr.cache[specifier.String()] = moduleCacheElement{err: err}
			return nil, err
		}
		return mr.resolveLoaded(basePWD, arg, data)
	}
}

// Imported returns the list of imported and resolved modules.
// Each string represents the path as used for importing.
func (mr *ModuleResolver) Imported() []string {
	if len(mr.cache) < 1 {
		return nil
	}
	modules := make([]string, 0, len(mr.cache))
	for name := range mr.cache {
		modules = append(modules, name)
	}
	return modules
}

func (mr *ModuleResolver) sobekModuleResolver(
	referencingScriptOrModule any, specifier string,
) (sobek.ModuleRecord, error) {
	return mr.resolve(mr.reversePath(referencingScriptOrModule), specifier)
}

func (mr *ModuleResolver) reversePath(referencingScriptOrModule interface{}) *url.URL {
	p, ok := mr.reverse[referencingScriptOrModule]
	if !ok {
		if referencingScriptOrModule != nil {
			panic("fix this")
		}
		return mr.base
	}

	if p.String() == "file:///-" {
		return mr.base
	}
	return p.JoinPath("..")
}

// ModuleSystem is implementing an ESM like module system to resolve js modules for k6 usage
type ModuleSystem struct {
	vu            VU
	instanceCache map[sobek.ModuleRecord]sobek.ModuleInstance
	resolver      *ModuleResolver
}

// NewModuleSystem returns a new ModuleSystem for the provide VU using the provided resoluter
func NewModuleSystem(resolver *ModuleResolver, vu VU) *ModuleSystem {
	rt := vu.Runtime()
	// TODO:figure out if we can remove this
	_ = rt.GlobalObject().DefineDataProperty("vubox",
		rt.ToValue(vubox{vu: vu}), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_FALSE)
	return &ModuleSystem{
		resolver:      resolver,
		instanceCache: make(map[sobek.ModuleRecord]sobek.ModuleInstance),
		vu:            vu,
	}
}

// RunSourceData runs the provided sourceData and adds it to the cache.
// If a module with the same specifier as the source is already cached
// it will be used instead of reevaluating the source from the provided SourceData.
//
// TODO: this API will likely change as native ESM support will likely not let us have the exports
// as one big sobek.Value that we can manipulate
func (ms *ModuleSystem) RunSourceData(source *loader.SourceData) (sobek.ModuleRecord, error) {
	specifier := source.URL.String()
	pwd := source.URL.JoinPath("../")
	if _, err := ms.resolver.resolveLoaded(pwd, specifier, source.Data); err != nil {
		return nil, err // TODO wrap as this should never happen
	}

	mod, err := ms.resolver.resolve(pwd, specifier)
	if err != nil {
		return nil, err // TODO wrap as this should never happen
	}
	err = mod.Link()
	if err != nil {
		return nil, err // TODO wrap as this should never happen
	}
	ci, ok := mod.(sobek.CyclicModuleRecord)
	if !ok {
		// TODO double check this works - this isn't really a case either way.
		return mod, nil
	}
	rt := ms.vu.Runtime()
	promise := rt.CyclicModuleRecordEvaluate(ci, ms.resolver.sobekModuleResolver)
	switch promise.State() {
	case sobek.PromiseStateRejected:
		return nil, promise.Result().Export().(error) //nolint:forcetypeassert
	case sobek.PromiseStateFulfilled:
		return mod, nil
	default:
		panic("TLA not supported in k6 at the moment") // TODO drop this by end of PR
	}
}

// ExportGloballyModule sets all exports of the provided module name on the globalThis.
// effectively making them globally available
func ExportGloballyModule(rt *sobek.Runtime, modSys *ModuleSystem, moduleName string) {
	m, err := modSys.resolver.resolve(nil, moduleName)
	if err != nil {
		panic(err)
	}
	wm, ok := m.(*goModule)
	if !ok {
		panic("bad")
	}
	var gmi *goModuleInstance
	gmi, err = modSys.getModuleInstanceFromGoModule(rt, wm)
	if err != nil {
		panic(err)
	}
	exports := gmi.getDefaultExport().ToObject(rt)

	for _, key := range exports.Keys() {
		if err := rt.Set(key, exports.Get(key)); err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", key, err))
		}
	}
}
