package js

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/loader"
)

type module interface {
	Instantiate(vu modules.VU) moduleInstance
}

type moduleInstance interface {
	execute() error
	exports() *goja.Object
}
type moduleCacheElement struct {
	mod module
	err error
}

type moduleResolver struct {
	cache     map[string]moduleCacheElement
	goModules map[string]interface{}
	loadCJS   cjsModuleLoader
}

func newModuleResolution(goModules map[string]interface{}, loadCJS cjsModuleLoader) *moduleResolver {
	return &moduleResolver{goModules: goModules, cache: make(map[string]moduleCacheElement), loadCJS: loadCJS}
}

func (mr *moduleResolver) setMain(main *loader.SourceData, c *compiler.Compiler) error {
	mod, err := cjsmoduleFromString(main.URL, main.Data, c)
	mr.cache[main.URL.String()] = moduleCacheElement{mod: mod, err: err}
	return err
}

func (mr *moduleResolver) resolveSpecifier(basePWD *url.URL, arg string) (*url.URL, error) {
	specifier, err := loader.Resolve(basePWD, arg)
	if err != nil {
		return nil, err
	}
	return specifier, nil
}

func (mr *moduleResolver) requireModule(name string) (module, error) {
	mod, ok := mr.goModules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if m, ok := mod.(modules.Module); ok {
		return &goModule{Module: m}, nil
	}

	return &baseGoModule{mod: mod}, nil
}

func (mr *moduleResolver) resolve(basePWD *url.URL, arg string) (module, error) {
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

type moduleSystem struct {
	vu            modules.VU
	instanceCache map[module]moduleInstance
	resolver      *moduleResolver
}

func newModuleSystem(resolution *moduleResolver, vu modules.VU) *moduleSystem {
	return &moduleSystem{
		resolver:      resolution,
		instanceCache: make(map[module]moduleInstance),
		vu:            vu,
	}
}

// Require is called when a module/file needs to be loaded by a script
func (ms *moduleSystem) Require(pwd *url.URL, arg string) (*goja.Object, error) {
	mod, err := ms.resolver.resolve(pwd, arg)
	if err != nil {
		return nil, err
	}
	if instance, ok := ms.instanceCache[mod]; ok {
		return instance.exports(), nil
	}

	instance := mod.Instantiate(ms.vu)
	ms.instanceCache[mod] = instance
	if err = instance.execute(); err != nil {
		return nil, err
	}

	return instance.exports(), nil
}
