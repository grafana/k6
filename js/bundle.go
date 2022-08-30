package js

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	Filename *url.URL
	Source   string
	Module   goja.CyclicModuleRecord
	Options  lib.Options

	BaseInitContext *InitContext

	RuntimeOptions    lib.RuntimeOptions
	CompatibilityMode lib.CompatibilityMode // parsed value
	registry          *metrics.Registry

	exports map[string]goja.Callable

	cache    map[string]moduleCacheElement
	reverse  map[goja.ModuleRecord]*url.URL
	compiler *compiler.Compiler
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime        *goja.Runtime
	ModuleInstance goja.ModuleInstance

	// TODO: maybe just have a reference to the Bundle? or save and pass rtOpts?
	env map[string]string

	exports      map[string]goja.Callable
	moduleVUImpl *moduleVUImpl
}

func (bi *BundleInstance) getExported(name string) goja.Value {
	return bi.ModuleInstance.GetBindingValue(name)
}

type moduleCacheElement struct {
	err error
	m   goja.ModuleRecord
}

// TODO use the first argument
func (b *Bundle) resolveModule(ref interface{}, specifier string) (goja.ModuleRecord, error) {
	if specifier == "k6" || strings.HasPrefix(specifier, "k6/") {
		k, ok := b.cache[specifier]
		if ok {
			return k.m, k.err
		}
		mod, ok := b.BaseInitContext.modules[specifier]
		if !ok {
			return nil, fmt.Errorf("unknown module: %s", specifier)
		}
		b.cache[specifier] = moduleCacheElement{m: wrapGoModule(mod)}
		return b.cache[specifier].m, nil
	}
	// todo fix
	var pwd *url.URL
	var main bool
	if ref == nil {
		pwd = loader.Dir(b.Filename)
		main = true
	} else if mr, ok := ref.(goja.ModuleRecord); ok {
		pwd = loader.Dir(b.reverse[mr])
	} // TODO fix this for all other cases using ref and cache
	fileurl, err := loader.Resolve(pwd, specifier)
	if err != nil {
		return nil, err
	}
	originalspecifier := specifier //nolint:ifshort
	specifier = fileurl.String()
	if specifier == "file://-" {
		fileurl = b.Filename
		specifier = b.Filename.String()
	}
	k, ok := b.cache[specifier]
	if ok {
		return k.m, k.err
	}
	var data string
	if originalspecifier == b.Filename.String() {
		// this mostly exists for tests ... kind of
		data = b.Source
	} else {
		var resolvedsrc *loader.SourceData
		resolvedsrc, err = loader.Load(b.BaseInitContext.logger, b.BaseInitContext.filesystems, fileurl, originalspecifier)
		if err != nil {
			b.cache[specifier] = moduleCacheElement{err: err}
			return nil, err
		}
		data = string(resolvedsrc.Data)
	}

	ast, ismodule, err := b.compiler.Parse(data, specifier, false)
	if err != nil {
		b.cache[specifier] = moduleCacheElement{err: err}
		return nil, err
	}
	if !ismodule {
		/* TODO enable this and fix the message
		b.BaseInitContext.logger.WithField("specifier", specifier).Error(
			"A not module will be evaluated. This might not work great, please don't use commonjs")
		//*/
		prg, _, err := b.compiler.Compile(data, specifier, false)
		if err != nil { // TODO try something on top?
			b.cache[specifier] = moduleCacheElement{err: err}
			return nil, err
		}
		m := wrapCommonJS(prg, main)
		b.reverse[m] = fileurl
		b.cache[specifier] = moduleCacheElement{m: m}
		return m, nil
		// todo warning
		// todo implement wrapper
	}
	m, err := goja.ModuleFromAST(ast, b.resolveModule)
	b.reverse[m] = fileurl
	if err != nil {
		b.cache[specifier] = moduleCacheElement{err: err}
		return nil, err
	}
	b.cache[specifier] = moduleCacheElement{m: m}
	return m, nil
}

// NewBundle creates a new bundle from a source file and a filesystem.
func NewBundle(
	piState *lib.TestPreInitState, src *loader.SourceData, filesystems map[string]afero.Fs,
) (*Bundle, error) {
	compatMode, err := lib.ValidateCompatibilityMode(piState.RuntimeOptions.CompatibilityMode.String)
	if err != nil {
		return nil, err
	}

	// Compile sources, both ES5 and ES6 are supported.
	code := string(src.Data)
	c := compiler.New(piState.Logger)
	c.Options = compiler.Options{
		CompatibilityMode: compatMode,
		Strict:            true,
		SourceMapLoader:   generateSourceMapLoader(piState.Logger, filesystems),
	}
	rt := goja.New()
	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	bundle := Bundle{
		Filename:          src.URL,
		Source:            code,
		BaseInitContext:   NewInitContext(piState.Logger, rt, c, compatMode, filesystems, loader.Dir(src.URL)),
		RuntimeOptions:    piState.RuntimeOptions,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
		registry:          piState.Registry,
		cache:             make(map[string]moduleCacheElement),
		reverse:           make(map[goja.ModuleRecord]*url.URL),
		compiler:          c,
	}

	m, err := bundle.resolveModule(nil, src.URL.String())
	if err != nil {
		return nil, err
	}
	err = m.Link()
	if err != nil {
		return nil, err
	}
	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	bundle.Module = m.(goja.CyclicModuleRecord)

	var mi goja.ModuleInstance
	if mi, err = bundle.instantiate(piState.Logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	err = bundle.getExports(piState.Logger, mi, true)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

// NewBundleFromArchive creates a new bundle from an lib.Archive.
func NewBundleFromArchive(piState *lib.TestPreInitState, arc *lib.Archive) (*Bundle, error) {
	if arc.Type != "js" {
		return nil, fmt.Errorf("expected bundle type 'js', got '%s'", arc.Type)
	}

	rtOpts := piState.RuntimeOptions // copy the struct from the TestPreInitState
	if !rtOpts.CompatibilityMode.Valid {
		// `k6 run --compatibility-mode=whatever archive.tar` should override
		// whatever value is in the archive
		rtOpts.CompatibilityMode = null.StringFrom(arc.CompatibilityMode)
	}
	compatMode, err := lib.ValidateCompatibilityMode(rtOpts.CompatibilityMode.String)
	if err != nil {
		return nil, err
	}

	c := compiler.New(piState.Logger)
	c.Options = compiler.Options{
		Strict:            true,
		CompatibilityMode: compatMode,
		SourceMapLoader:   generateSourceMapLoader(piState.Logger, arc.Filesystems),
	}
	rt := goja.New()
	i := NewInitContext(piState.Logger, rt, c, compatMode, arc.Filesystems, arc.PwdURL)

	env := arc.Env
	if env == nil {
		// Older archives (<=0.20.0) don't have an "env" property
		env = make(map[string]string)
	}
	for k, v := range rtOpts.Env {
		env[k] = v
	}
	rtOpts.Env = env

	bundle := &Bundle{
		Filename:          arc.FilenameURL,
		Source:            string(arc.Data),
		Options:           arc.Options,
		BaseInitContext:   i,
		RuntimeOptions:    rtOpts,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
		registry:          piState.Registry,
		cache:             make(map[string]moduleCacheElement),
		reverse:           make(map[goja.ModuleRecord]*url.URL),
		compiler:          c,
	}
	m, err := bundle.resolveModule(nil, arc.FilenameURL.String())
	if err != nil {
		return nil, err
	}
	err = m.Link()
	if err != nil {
		return nil, err
	}

	bundle.Module = m.(goja.CyclicModuleRecord)
	var mi goja.ModuleInstance
	if mi, err = bundle.instantiate(piState.Logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	// Grab exported objects, but avoid overwriting options, which would
	// be initialized from the metadata.json at this point.
	err = bundle.getExports(piState.Logger, mi, false)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (b *Bundle) makeArchive() *lib.Archive {
	arc := &lib.Archive{
		Type:              "js",
		Filesystems:       b.BaseInitContext.filesystems,
		Options:           b.Options,
		FilenameURL:       b.Filename,
		Data:              []byte(b.Source),
		PwdURL:            b.BaseInitContext.pwd,
		Env:               make(map[string]string, len(b.RuntimeOptions.Env)),
		CompatibilityMode: b.CompatibilityMode.String(),
		K6Version:         consts.Version,
		Goos:              runtime.GOOS,
	}
	// Copy env so changes in the archive are not reflected in the source Bundle
	for k, v := range b.RuntimeOptions.Env {
		arc.Env[k] = v
	}

	return arc
}

// getExports validates and extracts exported objects
func (b *Bundle) getExports(logger logrus.FieldLogger, mi goja.ModuleInstance, options bool) error {
	for _, k := range b.Module.GetExportedNames() {
		v := mi.GetBindingValue(k)
		if fn, ok := goja.AssertFunction(v); ok && k != consts.Options {
			b.exports[k] = fn
			continue
		}
		switch k {
		case consts.Options:
			if !options {
				continue
			}
			data, err := json.Marshal(v.Export())
			if err != nil {
				return err
			}
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&b.Options); err != nil {
				if uerr := json.Unmarshal(data, &b.Options); uerr != nil {
					return uerr
				}
				logger.WithError(err).Warn("There were unknown fields in the options exported in the script")
			}
		case consts.SetupFn:
			return errors.New("exported 'setup' must be a function")
		case consts.TeardownFn:
			return errors.New("exported 'teardown' must be a function")
		}
	}

	if len(b.exports) == 0 {
		return errors.New("no exported functions in script")
	}

	return nil
}

// Instantiate creates a new runtime from this bundle.
func (b *Bundle) Instantiate(logger logrus.FieldLogger, vuID uint64) (*BundleInstance, error) {
	// Instantiate the bundle into a new VM using a bound init context. This uses a context with a
	// runtime, but no state, to allow module-provided types to function within the init context.
	vuImpl := &moduleVUImpl{runtime: goja.New()}
	init := newBoundInitContext(b.BaseInitContext, vuImpl)
	var mi goja.ModuleInstance
	var err error
	if mi, err = b.instantiate(logger, vuImpl.runtime, init, vuID); err != nil {
		return nil, err
	}

	rt := vuImpl.runtime
	bi := &BundleInstance{
		Runtime:        rt,
		exports:        make(map[string]goja.Callable),
		env:            b.RuntimeOptions.Env,
		moduleVUImpl:   vuImpl,
		ModuleInstance: mi,
	}

	for _, name := range b.Module.GetExportedNames() {
		v := mi.GetBindingValue(name)
		fn, ok := goja.AssertFunction(v)
		if ok {
			bi.exports[name] = fn
		}
	}
	var instErr error
	jsOptions := mi.GetBindingValue(consts.Options)
	if !(jsOptions == nil || goja.IsNull(jsOptions) || goja.IsUndefined(jsOptions)) {
		jsOptionsObj := jsOptions.ToObject(rt)
		b.Options.ForEachSpecified("json", func(key string, val interface{}) {
			if err := jsOptionsObj.Set(key, val); err != nil {
				instErr = err
			}
		})
	}
	return bi, instErr
}

// fix
type vugetter struct {
	vu *moduleVUImpl
}

func (v vugetter) get() *moduleVUImpl {
	return v.vu
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.
func (b *Bundle) instantiate(
	logger logrus.FieldLogger, rt *goja.Runtime, init *InitContext, vuID uint64,
) (goja.ModuleInstance, error) {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())

	env := make(map[string]string, len(b.RuntimeOptions.Env))
	for key, value := range b.RuntimeOptions.Env {
		env[key] = value
	}
	rt.Set("__ENV", env)
	rt.Set("__VU", vuID)
	_ = rt.Set("console", newConsole(logger))

	if b.CompatibilityMode == lib.CompatibilityModeExtended {
		rt.Set("global", rt.GlobalObject())
	}
	rt.GlobalObject().DefineDataProperty("vugetter",
		rt.ToValue(vugetter{init.moduleVUImpl}), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)

	initenv := &common.InitEnvironment{
		Logger:      logger,
		FileSystems: init.filesystems,
		CWD:         loader.Dir(b.Filename),
		Registry:    b.registry,
	}
	unbindInit := b.setInitGlobals(rt, init)
	init.moduleVUImpl.ctx = context.Background()
	init.moduleVUImpl.initEnv = initenv
	init.moduleVUImpl.eventLoop = eventloop.New(init.moduleVUImpl)
	var err error
	err = common.RunWithPanicCatching(logger, rt, func() error {
		return init.moduleVUImpl.eventLoop.Start(func() error {
			promise := rt.CyclicModuleRecordEvaluate(b.Module, b.resolveModule)
			switch promise.State() {
			case goja.PromiseStateRejected:
				return promise.Result().Export().(error)
			case goja.PromiseStateFulfilled:
				return nil
			default:
				panic("TLA not supported in k6 at the moment")
			}
		})
	})

	if err != nil {
		var exception *goja.Exception
		if errors.As(err, &exception) {
			err = &scriptException{inner: exception}
		}
		return nil, err
	}
	unbindInit()
	init.moduleVUImpl.ctx = nil
	init.moduleVUImpl.initEnv = nil

	// If we've already initialized the original VU init context, forbid
	// any subsequent VUs to open new files
	if vuID == 0 {
		init.allowOnlyOpenedFiles()
	}

	rt.SetRandSource(common.NewRandSource())

	return rt.GetModuleInstance(b.Module), nil
}

func (b *Bundle) getCurrentModuleScript(rt *goja.Runtime) goja.ModuleRecord {
	// TODO implement this correctly in goja https://262.ecma-international.org/12.0/#sec-getactivescriptormodule
	var parent string
	var buf [2]goja.StackFrame
	frames := rt.CaptureCallStack(2, buf[:0])
	parent = frames[1].SrcName()

	parentModule, _ := b.resolveModule(nil, parent)
	return parentModule
}

func (b *Bundle) setInitGlobals(rt *goja.Runtime, init *InitContext) (unset func()) {
	mustSet := func(k string, v interface{}) {
		if err := rt.Set(k, v); err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", k, err))
		}
	}
	mustSet("require", func(specifier string) (interface{}, error) {
		// TODO get this in a method ?!? or a pure function
		parentModule := b.getCurrentModuleScript(rt)
		m, err := b.resolveModule(parentModule, specifier)
		if err != nil {
			return nil, err
		}
		if wm, ok := m.(*wrappedGoModule); ok {
			// TODO go through evaluation
			return toESModuleExports(wm.m.NewModuleInstance(init.moduleVUImpl).Exports()), nil
		}
		err = m.Link()
		if err != nil {
			return nil, err
		}
		var promise *goja.Promise
		if c, ok := m.(goja.CyclicModuleRecord); ok {
			promise = rt.CyclicModuleRecordEvaluate(c, b.resolveModule)
		} else {
			panic("shouldn't happen")
		}
		switch promise.State() {
		case goja.PromiseStateRejected:
			err = promise.Result().Export().(error)
		case goja.PromiseStateFulfilled:
		default:
			panic("TLA not supported in k6 at the moment")
		}
		if err != nil {
			return nil, err
		}
		if cjs, ok := m.(*wrappedCommonJS); ok {
			return rt.GetModuleInstance(cjs).(*wrappedCommonJSInstance).exports, nil //nolint:forcetypeassert
		}
		return rt.NamespaceObjectFor(m), nil // TODO fix this probably needs to be more exteneted
	})
	mustSet("open", init.Open)
	return func() {
		mustSet("require", goja.Undefined())
		mustSet("open", goja.Undefined())
	}
}

func generateSourceMapLoader(logger logrus.FieldLogger, filesystems map[string]afero.Fs,
) func(path string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		u, err := url.Parse(path)
		if err != nil {
			return nil, err
		}
		data, err := loader.Load(logger, filesystems, u, path)
		if err != nil {
			return nil, err
		}
		return data.Data, nil
	}
}
