package modules

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/loader"
)

// Require is the actual call that implements require
func (ms *ModuleSystem) Require(specifier string) (*sobek.Object, error) {
	if err := ms.resolver.usage.Uint64("usage/require", 1); err != nil {
		ms.resolver.logger.WithError(err).Warn("couldn't report usage")
	}

	if specifier == "" {
		return nil, errors.New("require() can't be used with an empty specifier")
	}

	rt := ms.vu.Runtime()
	parentModuleStr := getCurrentModuleScript(ms.vu)

	parentModule, _ := ms.resolver.sobekModuleResolver(nil, parentModuleStr)
	m, err := ms.resolver.sobekModuleResolver(parentModule, specifier)
	if err != nil {
		return nil, err
	}
	if wm, ok := m.(*goModule); ok {
		var gmi *goModuleInstance
		gmi, err = ms.getModuleInstanceFromGoModule(wm)
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
	var promise *sobek.Promise
	if c, ok := m.(sobek.CyclicModuleRecord); ok {
		promise = rt.CyclicModuleRecordEvaluate(c, ms.resolver.sobekModuleResolver)
	} else {
		panic(fmt.Sprintf("expected sobek.CyclicModuleRecord, but for some reason got a %T", m))
	}
	promisesThenIgnore(rt, promise)
	switch promise.State() {
	case sobek.PromiseStateRejected:
		err = promise.Result().Export().(error) //nolint:forcetypeassert
	case sobek.PromiseStateFulfilled:
	default:
	}
	if err != nil {
		return nil, err
	}
	if cjs, ok := m.(*cjsModule); ok {
		return rt.GetModuleInstance(cjs).(*cjsModuleInstance).exports, nil //nolint:forcetypeassert
	}
	return rt.NamespaceObjectFor(m), nil
}

func (ms *ModuleSystem) getModuleInstanceFromGoModule(wm *goModule) (wmi *goModuleInstance, err error) {
	rt := ms.vu.Runtime()
	mi := rt.GetModuleInstance(wm)
	if mi == nil {
		err = wm.Link()
		if err != nil {
			return nil, err
		}
		promise := rt.CyclicModuleRecordEvaluate(wm, ms.resolver.sobekModuleResolver)
		switch promise.State() {
		case sobek.PromiseStateRejected:
			err = promise.Result().Export().(error) //nolint:forcetypeassert
		case sobek.PromiseStateFulfilled:
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
		panic(fmt.Sprintf("a goModule instance is of not goModuleInstance type (got %T). "+
			"This is a k6 bug please report it (https://github.com/grafana/k6/issues)", mi))
	}
	return gmi, nil
}

// Resolve returns what the provided specifier will get resolved to if it was to be imported
// To be used by other parts to get the path
func (ms *ModuleSystem) Resolve(mr sobek.ModuleRecord, specifier string) (*url.URL, error) {
	if specifier == "" {
		return nil, errors.New("require() can't be used with an empty specifier")
	}

	baseModuleURL := ms.resolver.reversePath(mr)
	return ms.resolver.resolveSpecifier(baseModuleURL, specifier)
}

// CurrentlyRequiredModule returns the module that is currently being required.
// It is mostly used for old and somewhat buggy behaviour of the `open` call
func (ms *ModuleSystem) CurrentlyRequiredModule() (*url.URL, error) {
	fileStr, err := getPreviousRequiringFile(ms.vu)
	if err != nil {
		return nil, err
	}

	var u *url.URL
	switch {
	// this works around windows URLs like file://C:/some/path
	// url.Parse will think of C: as hostname
	case strings.HasPrefix(fileStr, "file://"):
		u = new(url.URL)
		u.Scheme = "file"
		u.Path, err = url.PathUnescape(strings.TrimPrefix(fileStr, "file://"))
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(fileStr, "https://"):
		var err error
		u, err = url.Parse(fileStr)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("couldn't parse %q as module url - this is a k6 bug, "+
			"please report it (https://github.com/grafana/k6/issues)", fileStr)
	}
	return loader.Dir(u), nil
}

// ShouldWarnOnParentDirNotMatchingCurrentModuleParentDir is a helper function to figure out if the provided url
// is the same folder that an import will be to.
// It also checks if the modulesystem is locked which means we are past the first init context.
// If false is returned means that we are not in the init context or the path is matching - so no warning should be done
// If true, the returned path is the module path that it should be relative to - the one that is checked against.
func (ms *ModuleSystem) ShouldWarnOnParentDirNotMatchingCurrentModuleParentDir(vu VU, parentModulePwd *url.URL,
) (string, bool) {
	if ms.resolver.locked {
		return "", false
	}
	normalizePathToURL := func(path string) string {
		u, err := url.Parse(path)
		if err != nil {
			return path
		}
		return loader.Dir(u).String()
	}
	parentModuleDir := parentModulePwd.String()
	parentModuleStr2 := getCurrentModuleScript(vu)
	parentModuleStr2Dir := normalizePathToURL(parentModuleStr2)
	if parentModuleDir != parentModuleStr2Dir {
		return parentModuleStr2, true
	}
	return "", false
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
	result[jsDefaultExportIdentifier] = exp.Default
	// This is to interop with any code that is transpiled by Babel or any similar tool.
	result["__esModule"] = true

	return result
}

func getCurrentModuleScript(vu VU) string {
	rt := vu.Runtime()
	var parent string
	var buf [2]sobek.StackFrame
	frames := rt.CaptureCallStack(2, buf[:0])
	if len(frames) == 0 || frames[1].SrcName() == "file:///-" {
		return vu.InitEnv().CWD.JoinPath("./-").String()
	}
	parent = frames[1].SrcName()

	return parent
}

func getPreviousRequiringFile(vu VU) (string, error) {
	rt := vu.Runtime()
	var buf [1000]sobek.StackFrame
	frames := rt.CaptureCallStack(1000, buf[:0])

	for i, frame := range frames[1:] { // first one should be the current require
		// TODO have this precalculated automatically
		if frame.FuncName() == "go.k6.io/k6/internal/js.(*requireImpl).require-fm" {
			// we need to get the one *before* but as we skip the first one the index matches ;)
			result := frames[i].SrcName()
			if result == "file:///-" {
				return vu.InitEnv().CWD.JoinPath("./-").String(), nil
			}
			return result, nil
		}
	}
	// hopefully nobody is calling `require` with 1000 big stack :crossedfingers:
	if len(frames) == 1000 {
		return "", errors.New("stack too big")
	}

	// fallback
	result := frames[len(frames)-1].SrcName()
	if result == "file:///-" {
		return vu.InitEnv().CWD.JoinPath("./-").String(), nil
	}
	return result, nil
}

// sets the provided promise in such way as to ignore falures
// this is mostly needed as failures are handled separately and we do not want those to lead to stopping the event loop
func promisesThenIgnore(rt *sobek.Runtime, promise *sobek.Promise) {
	call, _ := sobek.AssertFunction(rt.ToValue(promise).ToObject(rt).Get("then"))
	handler := rt.ToValue(func(_ sobek.Value) {})
	_, _ = call(rt.ToValue(promise), handler, handler)
}
