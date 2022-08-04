package js

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"runtime"

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
	Program  *goja.Program
	Options  lib.Options

	BaseInitContext *InitContext

	RuntimeOptions    lib.RuntimeOptions
	CompatibilityMode lib.CompatibilityMode // parsed value
	registry          *metrics.Registry

	exports map[string]goja.Callable
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime *goja.Runtime

	// TODO: maybe just have a reference to the Bundle? or save and pass rtOpts?
	env map[string]string

	exports      map[string]goja.Callable
	moduleVUImpl *moduleVUImpl
	pgm          programWithSource
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
	pgm, _, err := c.Compile(code, src.URL.String(), false)
	if err != nil {
		return nil, err
	}
	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	rt := goja.New()
	bundle := Bundle{
		Filename:          src.URL,
		Source:            code,
		Program:           pgm,
		BaseInitContext:   NewInitContext(piState.Logger, rt, c, compatMode, filesystems, loader.Dir(src.URL)),
		RuntimeOptions:    piState.RuntimeOptions,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
		registry:          piState.Registry,
	}
	if err = bundle.instantiate(piState.Logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	err = bundle.getExports(piState.Logger, rt, true)
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
	pgm, _, err := c.Compile(string(arc.Data), arc.FilenameURL.String(), false)
	if err != nil {
		return nil, err
	}
	rt := goja.New()
	initctx := NewInitContext(piState.Logger, rt, c, compatMode, arc.Filesystems, arc.PwdURL)

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
		Program:           pgm,
		Options:           arc.Options,
		BaseInitContext:   initctx,
		RuntimeOptions:    rtOpts,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
		registry:          piState.Registry,
	}

	if err = bundle.instantiate(piState.Logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	// Grab exported objects, but avoid overwriting options, which would
	// be initialized from the metadata.json at this point.
	err = bundle.getExports(piState.Logger, rt, false)
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
func (b *Bundle) getExports(logger logrus.FieldLogger, rt *goja.Runtime, options bool) error {
	pgm := b.BaseInitContext.programs[b.Filename.String()] // this is the main script and it's always present
	exportsV := pgm.module.Get("exports")
	if goja.IsNull(exportsV) || goja.IsUndefined(exportsV) {
		return errors.New("exports must be an object")
	}
	exports := exportsV.ToObject(rt)

	for _, k := range exports.Keys() {
		v := exports.Get(k)
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
	if err := b.instantiate(logger, vuImpl.runtime, init, vuID); err != nil {
		return nil, err
	}

	rt := vuImpl.runtime
	pgm := init.programs[b.Filename.String()] // this is the main script and it's always present
	bi := &BundleInstance{
		Runtime:      rt,
		exports:      make(map[string]goja.Callable),
		env:          b.RuntimeOptions.Env,
		moduleVUImpl: vuImpl,
		pgm:          pgm,
	}

	// Grab any exported functions that could be executed. These were
	// already pre-validated in cmd.validateScenarioConfig(), just get them here.
	exports := pgm.module.Get("exports").ToObject(rt)
	for k := range b.exports {
		fn, _ := goja.AssertFunction(exports.Get(k))
		bi.exports[k] = fn
	}

	jsOptions := exports.Get("options")
	var jsOptionsObj *goja.Object
	if jsOptions == nil || goja.IsNull(jsOptions) || goja.IsUndefined(jsOptions) {
		jsOptionsObj = rt.NewObject()
		err := exports.Set("options", jsOptionsObj)
		if err != nil {
			return nil, fmt.Errorf("couldn't set exported options with merged values: %w", err)
		}
	} else {
		jsOptionsObj = jsOptions.ToObject(rt)
	}

	var instErr error
	b.Options.ForEachSpecified("json", func(key string, val interface{}) {
		if err := jsOptionsObj.Set(key, val); err != nil {
			instErr = err
		}
	})

	return bi, instErr
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.

func (b *Bundle) initializeProgramObject(rt *goja.Runtime, init *InitContext) programWithSource {
	pgm := programWithSource{
		pgm:     b.Program,
		src:     b.Source,
		exports: rt.NewObject(),
		module:  rt.NewObject(),
	}
	_ = pgm.module.Set("exports", pgm.exports)
	init.programs[b.Filename.String()] = pgm
	return pgm
}

func (b *Bundle) instantiate(logger logrus.FieldLogger, rt *goja.Runtime, init *InitContext, vuID uint64) (err error) {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())

	env := make(map[string]string, len(b.RuntimeOptions.Env))
	for key, value := range b.RuntimeOptions.Env {
		env[key] = value
	}
	rt.Set("__ENV", env)
	rt.Set("__VU", vuID)
	_ = rt.Set("console", newConsole(logger))

	if init.compatibilityMode == lib.CompatibilityModeExtended {
		rt.Set("global", rt.GlobalObject())
	}

	initenv := &common.InitEnvironment{
		Logger:      logger,
		FileSystems: init.filesystems,
		CWD:         init.pwd,
		Registry:    b.registry,
	}
	unbindInit := b.setInitGlobals(rt, init)
	init.moduleVUImpl.ctx = context.Background()
	init.moduleVUImpl.initEnv = initenv
	init.moduleVUImpl.eventLoop = eventloop.New(init.moduleVUImpl)
	pgm := b.initializeProgramObject(rt, init)

	err = common.RunWithPanicCatching(logger, rt, func() error {
		return init.moduleVUImpl.eventLoop.Start(func() error {
			f, errRun := rt.RunProgram(b.Program)
			if errRun != nil {
				return errRun
			}
			if call, ok := goja.AssertFunction(f); ok {
				if _, errRun = call(pgm.exports, pgm.module, pgm.exports); errRun != nil {
					return errRun
				}
				return nil
			}
			panic("Somehow a commonjs main module is not wrapped in a function")
		})
	})

	if err != nil {
		var exception *goja.Exception
		if errors.As(err, &exception) {
			err = &scriptException{inner: exception}
		}
		return err
	}
	exportsV := pgm.module.Get("exports")
	if goja.IsNull(exportsV) {
		return errors.New("exports must be an object")
	}
	pgm.exports = exportsV.ToObject(rt)
	init.programs[b.Filename.String()] = pgm
	unbindInit()
	init.moduleVUImpl.ctx = nil
	init.moduleVUImpl.initEnv = nil

	// If we've already initialized the original VU init context, forbid
	// any subsequent VUs to open new files
	if vuID == 0 {
		init.allowOnlyOpenedFiles()
	}

	rt.SetRandSource(common.NewRandSource())

	return nil
}

func (b *Bundle) setInitGlobals(rt *goja.Runtime, init *InitContext) (unset func()) {
	mustSet := func(k string, v interface{}) {
		if err := rt.Set(k, v); err != nil {
			panic(fmt.Errorf("failed to set '%s' global object: %w", k, err))
		}
	}
	mustSet("require", init.Require)
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
