package modules

import (
	"errors"
	"net/url"
	"strings"

	"github.com/grafana/sobek"
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

const issueLink = "https://github.com/grafana/k6/issues/3534"

// Require is the actual call that implements require
func (r *LegacyRequireImpl) Require(specifier string) (*sobek.Object, error) {
	// TODO remove this in the future when we address https://github.com/grafana/k6/issues/2674
	// This is currently needed as each time require is called we need to record it's new pwd
	// to be used if a require *or* open is used within the file as they are relative to the
	// latest call to require.
	//
	// This is *not* the actual require behaviour defined in commonJS as it is actually always relative
	// to the file it is in. This is unlikely to be an issue but this code is here to keep backwards
	// compatibility *for now*.
	//
	// With native ESM this won't even be possible as `require` might not be called - instead an import
	// might be used in which case we won't be able to be doing this hack. In that case we either will
	// need some goja specific helper or to use stack traces as goja_nodejs does.
	currentPWD := r.currentlyRequiredModule
	if specifier != "k6" && !strings.HasPrefix(specifier, "k6/") {
		defer func() {
			r.currentlyRequiredModule = currentPWD
		}()
		fileURL, err := loader.Resolve(r.currentlyRequiredModule, specifier)
		if err != nil {
			return nil, err
		}
		// In theory we can give that downwards, but this makes the code more tightly coupled
		// plus as explained above this will be removed in the future so the code reflects more
		// closely what will be needed then
		if fileURL.Scheme == "file" && !r.modules.resolver.locked {
			r.warnUserOnPathResolutionDifferences(specifier)
		}
		r.currentlyRequiredModule = loader.Dir(fileURL)
	}

	if specifier == "" {
		return nil, errors.New("require() can't be used with an empty specifier")
	}

	return r.modules.Require(currentPWD, specifier)
}

// CurrentlyRequiredModule returns the module that is currently being required.
// It is mostly used for old and somewhat buggy behaviour of the `open` call
func (r *LegacyRequireImpl) CurrentlyRequiredModule() url.URL {
	return *r.currentlyRequiredModule
}

func (r *LegacyRequireImpl) warnUserOnPathResolutionDifferences(specifier string) {
	normalizePathToURL := func(path string) (*url.URL, error) {
		u, err := url.Parse(path)
		if err != nil {
			return nil, err
		}
		return loader.Dir(u), nil
	}
	// Warn users on their require depending on the none standard k6 behaviour.
	logger := r.vu.InitEnv().Logger
	correct, err := normalizePathToURL(getCurrentModuleScript(r.vu))
	if err != nil {
		logger.Warningf("Couldn't get the \"correct\" path to resolve specifier %q against: %q"+
			"Please report to issue %s. "+
			"This will not currently break your script but it might mean that in the future it won't work",
			specifier, err, issueLink)
	} else if r.currentlyRequiredModule.String() != correct.String() {
		logger.Warningf("The \"wrong\" path (%q) and the path actually used by k6 (%q) to resolve %q are different. "+
			"Please report to issue %s. "+
			"This will not currently break your script but *WILL* in the future, please report this!!!",
			correct, r.currentlyRequiredModule, specifier, issueLink)
	}

	k6behaviourString, err := getPreviousRequiringFile(r.vu)
	if err != nil {
		logger.Warningf("Couldn't get the \"wrong\" path to resolve specifier %q against: %q"+
			"Please report to issue %s. "+
			"This will not currently break your script but it might mean that in the future it won't work",
			specifier, err, issueLink)
		return
	}
	k6behaviour, err := normalizePathToURL(k6behaviourString)
	if err != nil {
		logger.Warningf("Couldn't get the \"wrong\" path to resolve specifier %q against: %q"+
			"Please report to issue %s. "+
			"This will not currently break your script but it might mean that in the future it won't work",
			specifier, err, issueLink)
		return
	}
	if r.currentlyRequiredModule.String() != k6behaviour.String() {
		// this should always be equal, but check anyway to be certain we won't break something
		logger.Warningf("The \"wrong\" path (%q) and the path actually used by k6 (%q) to resolve %q are different. "+
			"Please report to issue %s. "+
			"This will not currently break your script but it might mean that in the future it won't work",
			k6behaviour, r.currentlyRequiredModule, specifier, issueLink)
	}
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
	return vu.InitEnv().CWD.JoinPath("./-").String(), nil
}
