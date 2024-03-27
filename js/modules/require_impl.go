package modules

import (
	"errors"
	"net/url"
	"strings"

	"github.com/dop251/goja"
	"go.k6.io/k6/loader"
)

// LegacyRequireImpl is a legacy implementation of `require()` that is not compatible with
// CommonJS as it loads modules relative to the currently required file,
// instead of relative to the file the `require()` is written in.
// See https://github.com/grafana/k6/issues/2674
type LegacyRequireImpl struct {
	vu                      VU
	modules                 *ModuleSystem
	currentlyRequiredModule *url.URL
}

// NewLegacyRequireImpl creates a new LegacyRequireImpl
func NewLegacyRequireImpl(vu VU, ms *ModuleSystem, pwd url.URL) *LegacyRequireImpl {
	return &LegacyRequireImpl{
		vu:                      vu,
		modules:                 ms,
		currentlyRequiredModule: &pwd,
	}
}

// Require is the actual call that implements require
func (r *LegacyRequireImpl) Require(specifier string) (*goja.Object, error) {
	// TODO remove this in the future when we address https://github.com/grafana/k6/issues/2674
	// This is currently needed as each time require is called we need to record it's new pwd
	// to be used if a require *or* open is used within the file as they are relative to the
	// latest call to require.
	// This is *not* the actual require behaviour defined in commonJS as it is actually always relative
	// to the file it is in. This is unlikely to be an issue but this code is here to keep backwards
	// compatibility *for now*.
	// With native ESM this won't even be possible as `require` might not be called - instead an import
	// might be used in which case we won't be able to be doing this hack. In that case we either will
	// need some goja specific helper or to use stack traces as goja_nodejs does.
	currentPWD := r.currentlyRequiredModule
	if specifier != "k6" && !strings.HasPrefix(specifier, "k6/") {
		defer func() {
			r.currentlyRequiredModule = currentPWD
		}()
		// In theory we can give that downwards, but this makes the code more tightly coupled
		// plus as explained above this will be removed in the future so the code reflects more
		// closely what will be needed then
		fileURL, err := loader.Resolve(r.currentlyRequiredModule, specifier)
		if err != nil {
			return nil, err
		}
		r.currentlyRequiredModule = loader.Dir(fileURL)
	}

	if specifier == "" {
		return nil, errors.New("require() can't be used with an empty specifier")
	}

	rt := r.vu.Runtime()
	// parentModule := getCurrentModuleScript(rt, r.modules.resolver.gojaModuleResolver)
	parentModuleStr := getCurrentModuleScript(rt)
	parentModuleStr2 := getPreviousRequiringFile(rt)
	if parentModuleStr != parentModuleStr2 {
		r.vu.InitEnv().Logger.Warnf("requiring %s got two different modulestr %q and %q\n",
			specifier, parentModuleStr, parentModuleStr2)
		parentModuleStr = parentModuleStr2
	}
	parentModule, _ := r.modules.resolver.gojaModuleResolver(nil, parentModuleStr)
	m, err := r.modules.resolver.gojaModuleResolver(parentModule, specifier)
	if err != nil {
		return nil, err
	}
	if wm, ok := m.(*goModule); ok {
		var gmi *goModuleInstance
		gmi, err = r.getModuleInstanceFromGoModule(rt, wm)
		if err != nil {
			return nil, err
		}
		exports := toESModuleExports(gmi.mi.Exports())
		return rt.ToValue(exports).ToObject(rt), nil
	}
	err = m.Link()
	if err != nil {
		return nil, err
	}
	var promise *goja.Promise
	if c, ok := m.(goja.CyclicModuleRecord); ok {
		promise = rt.CyclicModuleRecordEvaluate(c, r.modules.resolver.gojaModuleResolver)
	} else {
		panic("shouldn't happen")
	}
	switch promise.State() {
	case goja.PromiseStateRejected:
		err = promise.Result().Export().(error) //nolint:forcetypeassert
	case goja.PromiseStateFulfilled:
	default:
	}
	if err != nil {
		return nil, err
	}
	if cjs, ok := m.(*cjsModule); ok {
		return rt.GetModuleInstance(cjs).(*cjsModuleInstance).exports, nil //nolint:forcetypeassert
	}
	return rt.NamespaceObjectFor(m), nil // TODO fix this probably needs to be more exteneted
}

func (r *LegacyRequireImpl) getModuleInstanceFromGoModule(
	rt *goja.Runtime, wm *goModule,
) (wmi *goModuleInstance, err error) {
	mi := rt.GetModuleInstance(wm)
	if mi == nil {
		err = wm.Link()
		if err != nil {
			return nil, err
		}
		promise := rt.CyclicModuleRecordEvaluate(wm, r.modules.resolver.gojaModuleResolver)
		switch promise.State() {
		case goja.PromiseStateRejected:
			err = promise.Result().Export().(error) //nolint:forcetypeassert
		case goja.PromiseStateFulfilled:
		default:
			panic("TLA in go modules is not supported in k6 at the moment")
		}
		if err != nil {
			return nil, err
		}
		mi = rt.GetModuleInstance(wm)
	}
	gmi, ok := mi.(*goModuleInstance)
	if !ok {
		panic("a goModule instance is of not goModuleInstance type. This is a k6 bug please report!")
	}
	return gmi, nil
}

// CurrentlyRequiredModule returns the module that is currently being required.
// It is mostly used for old and somewhat buggy behaviour of the `open` call
func (r *LegacyRequireImpl) CurrentlyRequiredModule() *url.URL {
	fileStr := getPreviousRequiringFile(r.vu.Runtime())
	var u *url.URL
	switch {
	// this works around windows URLs like file://C:/some/path
	// url.Parse will think of C: as hostname
	case strings.HasPrefix(fileStr, "file://"):
		u = new(url.URL)
		u.Scheme = "file"
		u.Path = strings.TrimPrefix(fileStr, "file://")
	case strings.HasPrefix(fileStr, "https://"):
		var err error
		u, err = url.Parse(fileStr)
		if err != nil {
			panic(err)
		}
	default:
		panic(fileStr)
	}
	return loader.Dir(u)
}

func getCurrentModuleScript(rt *goja.Runtime) string {
	var parent string
	var buf [2]goja.StackFrame
	frames := rt.CaptureCallStack(2, buf[:0])
	parent = frames[1].SrcName()

	return parent
}

func getPreviousRequiringFile(rt *goja.Runtime) string {
	// TODO:replace CurrentlyRequiredModule with this
	// TODO:stop needing either one https://github.com/grafana/k6/issues/2674
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

func toESModuleExports(exp Exports) interface{} {
	if exp.Named == nil {
		return exp.Default
	}
	if exp.Default == nil {
		return exp.Named
	}

	result := make(map[string]interface{}, len(exp.Named)+2)

	for k, v := range exp.Named {
		result[k] = v
	}
	// Maybe check that those weren't set
	result["default"] = exp.Default
	// this so babel works with the `default` when it transpiles from ESM to commonjs.
	// This should probably be removed once we have support for ESM directly. So that require doesn't get support for
	// that while ESM has.
	result["__esModule"] = true

	return result
}
