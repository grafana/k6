package modules

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/loader"
)

type module interface {
	instantiate(vu VU) moduleInstance
}

type moduleInstance interface {
	execute() error
	exports() *goja.Object
}
type moduleCacheElement struct {
	mod module
	err error
}

// ModuleResolver knows how to get base Module that can be initialized
type ModuleResolver struct {
	cache     map[string]moduleCacheElement
	goModules map[string]interface{}
	loadCJS   CJSModuleLoader

	// TODO: figure out a way to not have to have this
	mainpwd       *url.URL
	mainSpecifier string
}

// NewModuleResolver returns a new module resolution instance that will resolve.
// goModules is map of import file to a go module
// loadCJS is used to load commonjs files
func NewModuleResolver(goModules map[string]interface{}, loadCJS CJSModuleLoader) *ModuleResolver {
	return &ModuleResolver{goModules: goModules, cache: make(map[string]moduleCacheElement), loadCJS: loadCJS}
}

// SetMain sets what is the main module/script for this resolver
// TODO: this likely will change with ESM support
func (mr *ModuleResolver) SetMain(main *loader.SourceData, c *compiler.Compiler) error {
	mr.mainSpecifier = main.URL.String()
	mr.mainpwd = main.URL.ResolveReference(&url.URL{Path: "../"}) // TODO: fix
	mod, err := CJSModuleFromString(main.URL, main.Data, c)
	mr.cache[main.URL.String()] = moduleCacheElement{mod: mod, err: err}
	return err
}

func (mr *ModuleResolver) resolveSpecifier(basePWD *url.URL, arg string) (*url.URL, error) {
	specifier, err := loader.Resolve(basePWD, arg)
	if err != nil {
		return nil, err
	}
	return specifier, nil
}

func (mr *ModuleResolver) requireModule(name string) (module, error) {
	mod, ok := mr.goModules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if m, ok := mod.(Module); ok {
		return &goModule{Module: m}, nil
	}

	return &baseGoModule{mod: mod}, nil
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
		// Fall back to loading from the filesystem.
		mod, err := mr.loadCJS(specifier, arg)
		mr.cache[specifier.String()] = moduleCacheElement{mod: mod, err: err}
		return mod, err
	}
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
func (ms *ModuleSystem) Require(pwd *url.URL, arg string) (*goja.Object, error) {
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

// RunMain runs the main module and returns it exports
// TODO: this API will likely change as native ESM support will likely not let us have the exports
// as one big goja.Value that we can manipulate
func (ms *ModuleSystem) RunMain() (goja.Value, error) {
	mod, err := ms.resolver.resolve(ms.resolver.mainpwd, ms.resolver.mainSpecifier)
	if err != nil {
		return nil, err // TODO wrap as this should never happen
	}
	instance := mod.instantiate(ms.vu)
	err = instance.execute()
	if err != nil {
		return nil, err
	}
	return instance.exports(), err
}
