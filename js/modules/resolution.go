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

type module interface {
	instantiate(vu VU) moduleInstance
}

type moduleInstance interface {
	execute() error
	exports() *sobek.Object
}
type moduleCacheElement struct {
	mod module
	err error
}

// ModuleResolver knows how to get base Module that can be initialized
type ModuleResolver struct {
	cache     map[string]moduleCacheElement
	goModules map[string]interface{}
	loadCJS   FileLoader
	compiler  *compiler.Compiler
	locked    bool
}

// NewModuleResolver returns a new module resolution instance that will resolve.
// goModules is map of import file to a go module
// loadCJS is used to load commonjs files
func NewModuleResolver(goModules map[string]interface{}, loadCJS FileLoader, c *compiler.Compiler) *ModuleResolver {
	return &ModuleResolver{
		goModules: goModules,
		cache:     make(map[string]moduleCacheElement),
		loadCJS:   loadCJS,
		compiler:  c,
	}
}

func (mr *ModuleResolver) resolveSpecifier(basePWD *url.URL, arg string) (*url.URL, error) {
	specifier, err := loader.Resolve(basePWD, arg)
	if err != nil {
		return nil, err
	}
	return specifier, nil
}

func (mr *ModuleResolver) requireModule(name string) (module, error) {
	if mr.locked {
		return nil, fmt.Errorf(notPreviouslyResolvedModule, name)
	}
	mod, ok := mr.goModules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if m, ok := mod.(Module); ok {
		return &goModule{Module: m}, nil
	}

	return &baseGoModule{mod: mod}, nil
}

func (mr *ModuleResolver) resolveLoaded(basePWD *url.URL, arg string, data []byte) (module, error) {
	specifier, err := mr.resolveSpecifier(basePWD, arg)
	if err != nil {
		return nil, err
	}
	// try cache with the final specifier
	if cached, ok := mr.cache[specifier.String()]; ok {
		return cached.mod, cached.err
	}

	mod, err := cjsModuleFromString(specifier, data, mr.compiler)
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

func (mr *ModuleResolver) resolve(basePWD *url.URL, arg string) (module, error) {
	if cached, ok := mr.cache[arg]; ok {
		return cached.mod, cached.err
	}
	switch {
	case arg == "k6", strings.HasPrefix(arg, "k6/"):
		// Builtin or external modules ("k6", "k6/*", or "k6/x/*") are handled
		// specially, as they don't exist on the filesystem.
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
		mod, err := cjsModuleFromString(specifier, data, mr.compiler)
		mr.cache[specifier.String()] = moduleCacheElement{mod: mod, err: err}

		return mod, err
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

// ModuleSystem is implementing an ESM like module system to resolve js modules for k6 usage
type ModuleSystem struct {
	vu            VU
	instanceCache map[module]moduleInstance
	resolver      *ModuleResolver
}

// NewModuleSystem returns a new ModuleSystem for the provide VU using the provided resoluter
func NewModuleSystem(resolver *ModuleResolver, vu VU) *ModuleSystem {
	return &ModuleSystem{
		resolver:      resolver,
		instanceCache: make(map[module]moduleInstance),
		vu:            vu,
	}
}

// Require is called when a module/file needs to be loaded by a script
func (ms *ModuleSystem) Require(pwd *url.URL, arg string) (*sobek.Object, error) {
	mod, err := ms.resolver.resolve(pwd, arg)
	if err != nil {
		return nil, err
	}
	if instance, ok := ms.instanceCache[mod]; ok {
		return instance.exports(), nil
	}

	instance := mod.instantiate(ms.vu)
	ms.instanceCache[mod] = instance
	if err = instance.execute(); err != nil {
		return nil, err
	}

	return instance.exports(), nil
}

// RunSourceData runs the provided sourceData and adds it to the cache.
// If a module with the same specifier as the source is already cached
// it will be used instead of reevaluating the source from the provided SourceData.
//
// TODO: this API will likely change as native ESM support will likely not let us have the exports
// as one big sobek.Value that we can manipulate
func (ms *ModuleSystem) RunSourceData(source *loader.SourceData) (sobek.Value, error) {
	specifier := source.URL.String()
	pwd := source.URL.JoinPath("../")
	if _, err := ms.resolver.resolveLoaded(pwd, specifier, source.Data); err != nil {
		return nil, err // TODO wrap as this should never happen
	}
	return ms.Require(pwd, specifier)
}

// ExportGloballyModule sets all exports of the provided module name on the globalThis.
// effectively making them globally available
func ExportGloballyModule(rt *sobek.Runtime, modSys *ModuleSystem, moduleName string) {
	t, _ := modSys.Require(nil, moduleName)

	for _, key := range t.Keys() {
		if err := rt.Set(key, t.Get(key)); err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", key, err))
		}
	}
}
