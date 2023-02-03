package js

import (
	"fmt"
	"net/url"
	"path"
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

type modulesResolution struct {
	cache     map[string]moduleCacheElement
	goModules map[string]interface{}
}

func newModuleResolution(goModules map[string]interface{}) *modulesResolution {
	return &modulesResolution{goModules: goModules, cache: make(map[string]moduleCacheElement)}
}

type moduleSystem struct {
	vu            modules.VU
	instanceCache map[module]moduleInstance
	resolution    *modulesResolution
}

func newModuleSystem(resolution *modulesResolution, vu modules.VU) *moduleSystem {
	return &moduleSystem{
		resolution:    resolution,
		instanceCache: make(map[module]moduleInstance),
		vu:            vu,
	}
}

func (m *modulesResolution) setMain(main *loader.SourceData, c *compiler.Compiler) error {
	mod, err := cjsmoduleFromString(main.URL, main.Data, c)
	m.cache[main.URL.String()] = moduleCacheElement{mod: mod, err: err}
	return err
}

// Require is called when a module/file needs to be loaded by a script
func (i *moduleSystem) Require(pwd *url.URL, arg string, loadCJS cjsModuleLoader) (*goja.Object, error) {
	mod, err := i.resolve(pwd, arg, loadCJS)
	if err != nil {
		return nil, err
	}
	if instance, ok := i.instanceCache[mod]; ok {
		return instance.exports(), nil
	}

	instance := mod.Instantiate(i.vu)
	i.instanceCache[mod] = instance
	if err = instance.execute(); err != nil {
		return nil, err
	}

	return instance.exports(), nil
}

func (i *moduleSystem) resolve(basePWD *url.URL, arg string, loadCJS cjsModuleLoader) (module, error) {
	if cached, ok := i.resolution.cache[arg]; ok {
		return cached.mod, cached.err
	}
	switch {
	case arg == "k6", strings.HasPrefix(arg, "k6/"):
		// Builtin or external modules ("k6", "k6/*", or "k6/x/*") are handled
		// specially, as they don't exist on the filesystem.
		mod, err := i.resolution.requireModule(arg)
		i.resolution.cache[arg] = moduleCacheElement{mod: mod, err: err}
		return mod, err
	default:
		specifier, err := i.resolution.resolveSpecifier(basePWD, i.vu.Runtime(), arg)
		if err != nil {
			return nil, err
		}
		// try cache with the final specifier
		if cached, ok := i.resolution.cache[specifier.String()]; ok {
			return cached.mod, cached.err
		}
		// Fall back to loading from the filesystem.
		mod, err := loadCJS(specifier, arg)
		i.resolution.cache[specifier.String()] = moduleCacheElement{mod: mod, err: err}
		return mod, err
	}
}

func (i *modulesResolution) resolveSpecifier(basePWD *url.URL, rt *goja.Runtime, arg string) (*url.URL, error) {
	pwd, err := getPWDOfRequiringFile(basePWD, rt)
	if err != nil {
		return nil, err // TODO wrap
	}

	specifier, err := i.resolveFile(pwd, arg)
	if err != nil {
		return nil, err
	}
	return specifier, nil
}

func (i *modulesResolution) requireModule(name string) (module, error) {
	mod, ok := i.goModules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if m, ok := mod.(modules.Module); ok {
		return &goModule{Module: m}, nil
	}

	return &baseGoModule{mod: mod}, nil
}

func (i *modulesResolution) resolveFile(pwd *url.URL, name string) (*url.URL, error) {
	// Resolve the file path, push the target directory as pwd to make relative imports work.
	fileURL, err := loader.Resolve(pwd, name)
	if err != nil {
		return nil, err
	}

	return fileURL, nil
}

func getCurrentModulePath(rt *goja.Runtime) string {
	var buf [2]goja.StackFrame
	frames := rt.CaptureCallStack(2, buf[:0])
	if len(frames) < 2 {
		return "."
	}
	return frames[1].SrcName()
}

func getPreviousRequiringFile(rt *goja.Runtime) string {
	var buf [1000]goja.StackFrame
	frames := rt.CaptureCallStack(1000, buf[:0])

	for i, frame := range frames[1:] { // first one should be the current require
		// TODO have this precalculated automatically
		if frame.FuncName() == "go.k6.io/k6/js.(*requireImpl).require-fm" {
			// we need to get the one *before* but as we skip the first one the index matches ;)
			return frames[i].SrcName()
		}
	}
	// hopefully nobody is calling `require` with 1000 big stack :crossedfingers
	if len(frames) == 1000 {
		panic("stack too big")
	}

	// fallback
	return frames[len(frames)-1].SrcName()
}

func getPWDOfRequiringFile(fallback *url.URL, rt *goja.Runtime) (*url.URL, error) {
	pwd, err := url.Parse(getPreviousRequiringFile(rt))
	if err != nil {
		return nil, err // TODO wrap
	}
	if pwd.Host == "-" {
		pwd = fallback
	} else {
		pwd.Path = path.Dir(pwd.Path)
	}
	return pwd, nil
}
