package modules

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6/loader"
)

// Require is the actual call that implements require
func (ms *ModuleSystem) Require(specifier string) (*sobek.Object, error) {
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
		gmi, err = ms.getModuleInstanceFromGoModule(rt, wm)
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
		panic("shouldn't happen")
	}
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
	return rt.NamespaceObjectFor(m), nil // TODO fix this probably needs to be more exteneted
}

func (ms *ModuleSystem) getModuleInstanceFromGoModule(
	rt *sobek.Runtime, wm *goModule,
) (wmi *goModuleInstance, err error) {
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
		panic("a goModule instance is of not goModuleInstance type. This is a k6 bug please report!")
	}
	return gmi, nil
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
		return nil, fmt.Errorf("couldn't parse %q as module url - this is a k6 bug, please report it", fileStr)
	}
	return loader.Dir(u), nil
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
		if frame.FuncName() == "go.k6.io/k6/js.(*requireImpl).require-fm" {
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
