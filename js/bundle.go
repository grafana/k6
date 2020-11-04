/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js

import (
	"context"
	"encoding/json"
	"net/url"
	"runtime"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/compiler"
	jslib "github.com/loadimpact/k6/js/lib"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/loader"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	Filename *url.URL
	Source   string
	Program  *goja.Program
	Options  lib.Options

	BaseInitContext *InitContext

	Env               map[string]string
	CompatibilityMode lib.CompatibilityMode

	exports map[string]goja.Callable
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime *goja.Runtime
	Context *context.Context

	// TODO: maybe just have a reference to the Bundle? or save and pass rtOpts?
	env map[string]string

	exports map[string]goja.Callable
}

// NewBundle creates a new bundle from a source file and a filesystem.
func NewBundle(
	logger logrus.FieldLogger, src *loader.SourceData, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions,
) (*Bundle, error) {
	compatMode, err := lib.ValidateCompatibilityMode(rtOpts.CompatibilityMode.String)
	if err != nil {
		return nil, err
	}

	// Compile sources, both ES5 and ES6 are supported.
	code := string(src.Data)
	c := compiler.New(logger)
	pgm, _, err := c.Compile(code, src.URL.String(), "", "", true, compatMode)
	if err != nil {
		return nil, err
	}
	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	rt := goja.New()
	bundle := Bundle{
		Filename: src.URL,
		Source:   code,
		Program:  pgm,
		BaseInitContext: NewInitContext(logger, rt, c, compatMode, new(context.Context),
			filesystems, loader.Dir(src.URL)),
		Env:               rtOpts.Env,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
	}
	if err = bundle.instantiate(logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	err = bundle.getExports(rt, true)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

// NewBundleFromArchive creates a new bundle from an lib.Archive.
func NewBundleFromArchive(logger logrus.FieldLogger, arc *lib.Archive, rtOpts lib.RuntimeOptions) (*Bundle, error) {
	if arc.Type != "js" {
		return nil, errors.Errorf("expected bundle type 'js', got '%s'", arc.Type)
	}

	compatModeStr := arc.CompatibilityMode
	if rtOpts.CompatibilityMode.Valid {
		// `k6 run --compatibility-mode=whatever archive.tar` should  override
		// whatever value is in the archive
		compatModeStr = rtOpts.CompatibilityMode.String
	}
	compatMode, err := lib.ValidateCompatibilityMode(compatModeStr)
	if err != nil {
		return nil, err
	}

	c := compiler.New(logger)
	pgm, _, err := c.Compile(string(arc.Data), arc.FilenameURL.String(), "", "", true, compatMode)
	if err != nil {
		return nil, err
	}
	rt := goja.New()
	initctx := NewInitContext(logger, rt, c, compatMode,
		new(context.Context), arc.Filesystems, arc.PwdURL)

	env := arc.Env
	if env == nil {
		// Older archives (<=0.20.0) don't have an "env" property
		env = make(map[string]string)
	}
	for k, v := range rtOpts.Env {
		env[k] = v
	}

	bundle := &Bundle{
		Filename:          arc.FilenameURL,
		Source:            string(arc.Data),
		Program:           pgm,
		Options:           arc.Options,
		BaseInitContext:   initctx,
		Env:               env,
		CompatibilityMode: compatMode,
		exports:           make(map[string]goja.Callable),
	}

	if err = bundle.instantiate(logger, rt, bundle.BaseInitContext, 0); err != nil {
		return nil, err
	}

	// Grab exported objects, but avoid overwriting options, which would
	// be initialized from the metadata.json at this point.
	err = bundle.getExports(rt, false)
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
		Env:               make(map[string]string, len(b.Env)),
		CompatibilityMode: b.CompatibilityMode.String(),
		K6Version:         consts.Version,
		Goos:              runtime.GOOS,
	}
	// Copy env so changes in the archive are not reflected in the source Bundle
	for k, v := range b.Env {
		arc.Env[k] = v
	}

	return arc
}

// getExports validates and extracts exported objects
func (b *Bundle) getExports(rt *goja.Runtime, options bool) error {
	exportsV := rt.Get("exports")
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
			if err := json.Unmarshal(data, &b.Options); err != nil {
				return err
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
func (b *Bundle) Instantiate(logger logrus.FieldLogger, vuID int64) (bi *BundleInstance, instErr error) {
	// TODO: actually use a real context here, so that the instantiation can be killed
	// Placeholder for a real context.
	ctxPtr := new(context.Context)

	// Instantiate the bundle into a new VM using a bound init context. This uses a context with a
	// runtime, but no state, to allow module-provided types to function within the init context.
	rt := goja.New()
	init := newBoundInitContext(b.BaseInitContext, ctxPtr, rt)
	if err := b.instantiate(logger, rt, init, vuID); err != nil {
		return nil, err
	}

	bi = &BundleInstance{
		Runtime: rt,
		Context: ctxPtr,
		exports: make(map[string]goja.Callable),
		env:     b.Env,
	}

	// Grab any exported functions that could be executed. These were
	// already pre-validated in cmd.validateScenarioConfig(), just get them here.
	exports := rt.Get("exports").ToObject(rt)
	for k := range b.exports {
		fn, _ := goja.AssertFunction(exports.Get(k))
		bi.exports[k] = fn
	}

	jsOptions := rt.Get("options")
	var jsOptionsObj *goja.Object
	if jsOptions == nil || goja.IsNull(jsOptions) || goja.IsUndefined(jsOptions) {
		jsOptionsObj = rt.NewObject()
		rt.Set("options", jsOptionsObj)
	} else {
		jsOptionsObj = jsOptions.ToObject(rt)
	}
	b.Options.ForEachSpecified("json", func(key string, val interface{}) {
		if err := jsOptionsObj.Set(key, val); err != nil {
			instErr = err
		}
	})

	return bi, instErr
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.
func (b *Bundle) instantiate(logger logrus.FieldLogger, rt *goja.Runtime, init *InitContext, vuID int64) error {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())

	if init.compatibilityMode == lib.CompatibilityModeExtended {
		if _, err := rt.RunProgram(jslib.GetCoreJS()); err != nil {
			return err
		}
	}

	exports := rt.NewObject()
	rt.Set("exports", exports)
	module := rt.NewObject()
	_ = module.Set("exports", exports)
	rt.Set("module", module)

	env := make(map[string]string, len(b.Env))
	for key, value := range b.Env {
		env[key] = value
	}
	rt.Set("__ENV", env)
	rt.Set("__VU", vuID)
	rt.Set("console", common.Bind(rt, newConsole(logger), init.ctxPtr))

	// TODO: get rid of the unused ctxPtr, use a real external context (so we
	// can interrupt), build the common.InitEnvironment earlier and reuse it
	initenv := &common.InitEnvironment{
		Logger:      logger,
		FileSystems: init.filesystems,
		CWD:         init.pwd,
	}
	ctx := common.WithInitEnv(context.Background(), initenv)
	*init.ctxPtr = common.WithRuntime(ctx, rt)
	unbindInit := common.BindToGlobal(rt, common.Bind(rt, init, init.ctxPtr))
	if _, err := rt.RunProgram(b.Program); err != nil {
		return err
	}
	unbindInit()
	*init.ctxPtr = nil

	rt.SetRandSource(common.NewRandSource())

	return nil
}
