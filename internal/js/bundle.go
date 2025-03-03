package js

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/js/compiler"
	"go.k6.io/k6/internal/js/eventloop"
	"go.k6.io/k6/internal/js/modules/k6/webcrypto"
	"go.k6.io/k6/internal/js/tc55/timers"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/fsext"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	sourceData *loader.SourceData
	Options    lib.Options

	CompatibilityMode lib.CompatibilityMode // parsed value
	preInitState      *lib.TestPreInitState

	filesystems map[string]fsext.Fs
	pwd         *url.URL

	callableExports map[string]struct{}
	ModuleResolver  *modules.ModuleResolver
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime *sobek.Runtime

	// TODO: maybe just have a reference to the Bundle? or save and pass rtOpts?
	env map[string]string

	mainModule   sobek.ModuleRecord
	moduleVUImpl *moduleVUImpl
}

func (bi *BundleInstance) getCallableExport(name string) sobek.Callable {
	fn, ok := sobek.AssertFunction(bi.getExported(name))
	_ = ok // TODO maybe return it
	return fn
}

func (bi *BundleInstance) getExported(name string) sobek.Value {
	re, ambigiuous := bi.mainModule.ResolveExport(name)
	if ambigiuous || re == nil {
		return nil
	}
	moduleInstance := bi.Runtime.GetModuleInstance(re.Module)
	if moduleInstance == nil {
		panic(fmt.Sprintf("couldn't load module instance while resolving identifier %q - this is a k6 bug "+
			", please report it (https://github.com/grafana/k6/issues)", re.BindingName))
	}

	return moduleInstance.GetBindingValue(re.BindingName)
}

// NewBundle creates a new bundle from a source file and a filesystem.
func NewBundle(
	piState *lib.TestPreInitState, src *loader.SourceData, filesystems map[string]fsext.Fs,
) (*Bundle, error) {
	return newBundle(piState, src, filesystems, lib.Options{}, true)
}

func newBundle(
	piState *lib.TestPreInitState, src *loader.SourceData, filesystems map[string]fsext.Fs,
	options lib.Options, updateOptions bool, // TODO: try to figure out a way to not need both
) (*Bundle, error) {
	compatMode, err := lib.ValidateCompatibilityMode(piState.RuntimeOptions.CompatibilityMode.String)
	if err != nil {
		return nil, err
	}

	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	bundle := &Bundle{
		sourceData:        src,
		Options:           options,
		CompatibilityMode: compatMode,
		callableExports:   make(map[string]struct{}),
		filesystems:       filesystems,
		pwd:               src.PWD,
		preInitState:      piState,
	}

	if bundle.pwd == nil {
		bundle.pwd = loader.Dir(src.URL)
	}

	c := bundle.newCompiler(piState.Logger)
	bundle.ModuleResolver = modules.NewModuleResolver(
		getJSModules(), generateFileLoad(bundle), c, bundle.pwd, piState.Usage, piState.Logger)

	// Instantiate the bundle into a new VM using a bound init context. This uses a context with a
	// runtime, but no state, to allow module-provided types to function within the init context.
	// TODO use a real context
	vuImpl := &moduleVUImpl{
		ctx:     context.Background(),
		runtime: sobek.New(),
		events: events{
			global: piState.Events,
			local:  event.NewEventSystem(100, piState.Logger),
		},
	}
	vuImpl.eventLoop = eventloop.New(vuImpl)
	bi, err := bundle.instantiate(vuImpl, 0)
	if err != nil {
		return nil, err
	}
	bundle.ModuleResolver.Lock()

	err = bundle.populateExports(updateOptions, bi)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

// NewBundleFromArchive creates a new bundle from an lib.Archive.
func NewBundleFromArchive(piState *lib.TestPreInitState, arc *lib.Archive) (*Bundle, error) {
	if arc.Type != "js" {
		return nil, fmt.Errorf("expected bundle type 'js', got '%s'", arc.Type)
	}

	if !piState.RuntimeOptions.CompatibilityMode.Valid {
		// `k6 run --compatibility-mode=whatever archive.tar` should override
		// whatever value is in the archive
		piState.RuntimeOptions.CompatibilityMode = null.StringFrom(arc.CompatibilityMode)
	}
	env := arc.Env
	if env == nil {
		// Older archives (<=0.20.0) don't have an "env" property
		env = make(map[string]string)
	}
	for k, v := range piState.RuntimeOptions.Env {
		env[k] = v
	}
	piState.RuntimeOptions.Env = env

	return newBundle(piState, &loader.SourceData{
		Data: arc.Data,
		URL:  arc.FilenameURL,
	}, arc.Filesystems, arc.Options, false)
}

func (b *Bundle) makeArchive() *lib.Archive {
	clonedSourceDataURL, _ := url.Parse(b.sourceData.URL.String())
	clonedPwdURL, _ := url.Parse(b.pwd.String())

	arc := &lib.Archive{
		Type:              "js",
		Filesystems:       b.filesystems,
		Options:           b.Options,
		FilenameURL:       clonedSourceDataURL,
		Data:              b.sourceData.Data,
		PwdURL:            clonedPwdURL,
		Env:               make(map[string]string, len(b.preInitState.RuntimeOptions.Env)),
		CompatibilityMode: b.CompatibilityMode.String(),
		K6Version:         build.Version,
		Goos:              runtime.GOOS,
	}
	// Copy env so changes in the archive are not reflected in the source Bundle
	for k, v := range b.preInitState.RuntimeOptions.Env {
		arc.Env[k] = v
	}

	return arc
}

// populateExports validates and extracts exported objects
func (b *Bundle) populateExports(updateOptions bool, bi *BundleInstance) error {
	var err error
	ch := make(chan struct{})
	bi.mainModule.GetExportedNames(func(names []string) {
		defer close(ch)
		for _, k := range names {
			v := bi.getExported(k)
			if _, ok := sobek.AssertFunction(v); ok && k != consts.Options {
				b.callableExports[k] = struct{}{}
				continue
			}
			switch k {
			case consts.Options:
				if !updateOptions || v == nil {
					continue
				}
				var data []byte
				data, err = json.Marshal(v.Export())
				if err != nil {
					err = fmt.Errorf("error parsing script options: %w", err)
					return
				}
				dec := json.NewDecoder(bytes.NewReader(data))
				dec.DisallowUnknownFields()
				if err = dec.Decode(&b.Options); err != nil {
					if uerr := json.Unmarshal(data, &b.Options); uerr != nil {
						err = errext.WithAbortReasonIfNone(
							errext.WithExitCodeIfNone(uerr, exitcodes.InvalidConfig),
							errext.AbortedByScriptError,
						)
						return
					}
					b.preInitState.Logger.WithError(err).Warn("There were unknown fields in the options exported in the script")
					err = nil
				}
			case consts.SetupFn:
				err = errors.New("exported 'setup' must be a function")
				return
			case consts.TeardownFn:
				err = errors.New("exported 'teardown' must be a function")
				return
			}
		}
	})
	<-ch
	if err != nil {
		return err
	}

	if len(b.callableExports) == 0 {
		return errors.New("no exported functions in script")
	}

	return nil
}

// Instantiate creates a new runtime from this bundle.
func (b *Bundle) Instantiate(ctx context.Context, vuID uint64) (*BundleInstance, error) {
	// Instantiate the bundle into a new VM using a bound init context. This uses a context with a
	// runtime, but no state, to allow module-provided types to function within the init context.
	vuImpl := &moduleVUImpl{
		ctx:     ctx,
		runtime: sobek.New(),
		events: events{
			global: b.preInitState.Events,
			local:  event.NewEventSystem(100, b.preInitState.Logger),
		},
	}
	vuImpl.eventLoop = eventloop.New(vuImpl)
	bi, err := b.instantiate(vuImpl, vuID)
	if err != nil {
		return nil, err
	}
	if err = bi.manipulateOptions(b.Options); err != nil {
		return nil, err
	}

	return bi, nil
}

func (bi *BundleInstance) manipulateOptions(options lib.Options) error {
	// Grab any exported functions that could be executed. These were
	// already pre-validated in cmd.validateScenarioConfig(), just get them here.
	jsOptions := bi.getExported(consts.Options)
	var jsOptionsObj *sobek.Object
	if common.IsNullish(jsOptions) {
		return nil
	}

	jsOptionsObj = jsOptions.ToObject(bi.Runtime)
	var instErr error
	options.ForEachSpecified("json", func(key string, val interface{}) {
		if err := jsOptionsObj.Set(key, val); err != nil {
			instErr = err
		}
	})
	return instErr
}

func (b *Bundle) newCompiler(logger logrus.FieldLogger) *compiler.Compiler {
	c := compiler.New(logger)
	c.WithUsage(b.preInitState.Usage)
	c.Options = compiler.Options{
		CompatibilityMode: b.CompatibilityMode,
		SourceMapLoader:   generateSourceMapLoader(logger, b.filesystems),
	}
	return c
}

func (b *Bundle) instantiate(vuImpl *moduleVUImpl, vuID uint64) (*BundleInstance, error) {
	rt := vuImpl.runtime
	err := b.setupJSRuntime(rt, vuID, b.preInitState.Logger)
	if err != nil {
		return nil, err
	}

	initenv := &common.InitEnvironment{
		TestPreInitState: b.preInitState,
		FileSystems:      b.filesystems,
		CWD:              b.pwd,
	}

	modSys := modules.NewModuleSystem(b.ModuleResolver, vuImpl)
	b.setInitGlobals(rt, vuImpl, modSys)

	err = registerGlobals(vuImpl)
	if err != nil {
		return nil, err
	}

	vuImpl.initEnv = initenv
	defer func() {
		vuImpl.initEnv = nil
	}()

	// TODO: make something cleaner for interrupting scripts, and more unified
	// (e.g. as a part of the event loop?
	initDone := make(chan struct{})
	go func() {
		select {
		case <-vuImpl.ctx.Done():
			rt.Interrupt(vuImpl.ctx.Err())
		case initDone <- struct{}{}: // do nothing
		}
		close(initDone)
	}()

	bi := &BundleInstance{
		Runtime:      vuImpl.runtime,
		env:          b.preInitState.RuntimeOptions.Env,
		moduleVUImpl: vuImpl,
	}
	var result *modules.RunSourceDataResult
	callback := func() error { // this exists so that Sobek catches uncatchable panics such as Interrupt
		var err error
		result, err = modSys.RunSourceData(b.sourceData)
		return err
	}

	call, _ := sobek.AssertFunction(vuImpl.runtime.ToValue(callback))

	err = vuImpl.eventLoop.Start(func() error {
		_, err := call(nil)
		return err
	})

	<-initDone

	if err == nil {
		var finished bool
		bi.mainModule, finished, err = result.Result()
		if !finished {
			return nil, errors.New("initializing the main module hasn't finished, this is a bug in k6 please report it")
		}
	}

	if err != nil {
		var exception *sobek.Exception
		if errors.As(err, &exception) {
			err = &scriptExceptionError{inner: exception}
		}
		return nil, err
	}

	// If we've already initialized the original VU init context, forbid
	// any subsequent VUs to open new files
	if vuID == 0 {
		allowOnlyOpenedFiles(b.filesystems["file"])
	}

	rt.SetRandSource(common.NewRandSource())

	return bi, nil
}

// registerGlobals registers the globals for the runtime.
// e.g. timers and webcrypto.
func registerGlobals(vuImpl *moduleVUImpl) error {
	err := timers.SetupGlobally(vuImpl)
	if err != nil {
		return err
	}

	return webcrypto.SetupGlobally(vuImpl)
}

func (b *Bundle) setupJSRuntime(rt *sobek.Runtime, vuID uint64, logger logrus.FieldLogger) error {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())

	env := make(map[string]string, len(b.preInitState.RuntimeOptions.Env))
	for key, value := range b.preInitState.RuntimeOptions.Env {
		env[key] = value
	}
	err := rt.Set("__ENV", env)
	if err != nil {
		return err
	}
	err = rt.Set("__VU", vuID)
	if err != nil {
		return err
	}
	err = rt.Set("console", newConsole(logger))
	if err != nil {
		return err
	}

	if b.CompatibilityMode == lib.CompatibilityModeExtended {
		globalThis := rt.GlobalObject()
		err = globalThis.DefineAccessorProperty("global",
			rt.ToValue(func() sobek.Value {
				if err := b.preInitState.Usage.Uint64("usage/global", 1); err != nil {
					b.preInitState.Logger.WithError(err).Warn("couldn't report usage")
				}
				return globalThis
			}), rt.ToValue(func(newGlobal *sobek.Object) { // probably not a thing that will happen but still
				globalThis = newGlobal
			}),
			sobek.FLAG_TRUE, sobek.FLAG_TRUE)
		if err != nil {
			return err
		}
	}
	return nil
}

// this exists only to make the check in the init context.
type requireImpl struct {
	inInitContext func() bool
	modSys        *modules.ModuleSystem
}

func (r *requireImpl) require(specifier string) (*sobek.Object, error) {
	if !r.inInitContext() {
		return nil, fmt.Errorf(cantBeUsedOutsideInitContextMsg, "require")
	}
	return r.modSys.Require(specifier)
}

func (b *Bundle) setInitGlobals(rt *sobek.Runtime, vu *moduleVUImpl, modSys *modules.ModuleSystem) {
	mustSet := func(k string, v interface{}) {
		if err := rt.Set(k, v); err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", k, err))
		}
	}

	impl := requireImpl{
		inInitContext: func() bool { return vu.state == nil },
		modSys:        modSys,
	}

	mustSet("require", impl.require)

	mustSet("open", func(filename string, args ...string) (sobek.Value, error) {
		// TODO fix in stack traces
		if vu.state != nil {
			return nil, fmt.Errorf(cantBeUsedOutsideInitContextMsg, "open")
		}

		if filename == "" {
			return nil, errors.New("open() can't be used with an empty filename")
		}
		// This uses the pwd from the requireImpl
		pwd, err := modSys.CurrentlyRequiredModule()
		if err != nil {
			return nil, err
		}
		if !(strings.HasPrefix(filename, "file://") || filepath.IsAbs(filename)) {
			otherPath, shouldWarn := modSys.ShouldWarnOnParentDirNotMatchingCurrentModuleParentDir(vu, pwd)
			logger := b.preInitState.Logger
			if shouldWarn {
				logger.Warningf("open() was used and is currently relative to '%s', but in the future "+
					"it will be aligned with how `require` and imports work and will be relative to '%s'. This means "+
					"that in the future open will open relative path relative to the module/file it is written in. "+
					"You can future proof this by using `import.meta.resolve()` to get relative paths to the file it "+
					"is written in the current k6 version.", pwd, otherPath)
				err = b.preInitState.Usage.Uint64("deprecations/openRelativity", 1)
				if err != nil {
					logger.WithError(err).Warn("failed reporting usage of deprecated relativity of open()")
				}
			}
		}

		return openImpl(rt, b.filesystems["file"], pwd, filename, args...)
	})
	warnAboutModuleMixing := func(name string) {
		warnFunc := rt.ToValue(func() error {
			return fmt.Errorf(
				"you are trying to access identifier %q, this likely is due to mixing "+
					"ECMAScript Modules (ESM) and CommonJS syntax. "+
					"This isn't supported in the JavaScript standard, please use only one or the other",
				name)
		})
		err := rt.GlobalObject().DefineAccessorProperty(name, warnFunc, warnFunc, sobek.FLAG_FALSE, sobek.FLAG_FALSE)
		if err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", name, err))
		}
	}
	warnAboutModuleMixing("module")
	warnAboutModuleMixing("exports")

	rt.SetFinalImportMeta(func(o *sobek.Object, mr sobek.ModuleRecord) {
		err := o.Set("resolve", func(specifier string) (string, error) {
			u, err := modSys.Resolve(mr, specifier)
			if err != nil {
				return "", err
			}
			return u.String(), nil
		})
		if err != nil {
			panic("error while creating `import.meta.resolve`: " + err.Error())
		}
	})
}

func generateFileLoad(b *Bundle) modules.FileLoader {
	return func(specifier *url.URL, name string) ([]byte, error) {
		if filepath.IsAbs(name) && runtime.GOOS == "windows" {
			b.preInitState.Logger.Warnf("'%s' was imported with an absolute path - this won't be cross-platform and "+
				"won't work if you move the script between machines or run it with `k6 cloud`; if absolute paths are "+
				"required, import them with the `file://` schema for slightly better compatibility",
				name)
		}
		d, err := loader.Load(b.preInitState.Logger, b.filesystems, specifier, name)
		if err != nil {
			return nil, err
		}
		return d.Data, nil
	}
}
