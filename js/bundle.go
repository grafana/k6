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

	"github.com/loadimpact/k6/lib/consts"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/compiler"
	jslib "github.com/loadimpact/k6/js/lib"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	Filename *url.URL
	Source   string
	Program  *goja.Program
	Options  lib.Options

	BaseInitContext *InitContext

	Env map[string]string
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime *goja.Runtime
	Context *context.Context
	Default goja.Callable
}

// NewBundle creates a new bundle from a source file and a filesystem.
func NewBundle(src *loader.SourceData, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions) (*Bundle, error) {
	compiler, err := compiler.New()
	if err != nil {
		return nil, err
	}

	// Compile sources, both ES5 and ES6 are supported.
	code := string(src.Data)
	pgm, _, err := compiler.Compile(code, src.URL.String(), "", "", true)
	if err != nil {
		return nil, err
	}
	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	rt := goja.New()
	bundle := Bundle{
		Filename:        src.URL,
		Source:          code,
		Program:         pgm,
		BaseInitContext: NewInitContext(rt, compiler, new(context.Context), filesystems, loader.Dir(src.URL)),
		Env:             rtOpts.Env,
	}
	if err := bundle.instantiate(rt, bundle.BaseInitContext); err != nil {
		return nil, err
	}

	// Grab exports.
	exportsV := rt.Get("exports")
	if goja.IsNull(exportsV) || goja.IsUndefined(exportsV) {
		return nil, errors.New("exports must be an object")
	}
	exports := exportsV.ToObject(rt)

	// Validate the default function.
	def := exports.Get("default")
	if def == nil || goja.IsNull(def) || goja.IsUndefined(def) {
		return nil, errors.New("script must export a default function")
	}
	if _, ok := goja.AssertFunction(def); !ok {
		return nil, errors.New("default export must be a function")
	}

	// Extract/validate other exports.
	for _, k := range exports.Keys() {
		v := exports.Get(k)
		switch k {
		case "default": // Already checked above.
		case "options":
			data, err := json.Marshal(v.Export())
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &bundle.Options); err != nil {
				return nil, err
			}
		case "setup":
			if _, ok := goja.AssertFunction(v); !ok {
				return nil, errors.New("exported 'setup' must be a function")
			}
		case "teardown":
			if _, ok := goja.AssertFunction(v); !ok {
				return nil, errors.New("exported 'teardown' must be a function")
			}
		}
	}

	return &bundle, nil
}

// NewBundleFromArchive creates a new bundle from an lib.Archive.
func NewBundleFromArchive(arc *lib.Archive, rtOpts lib.RuntimeOptions) (*Bundle, error) {
	compiler, err := compiler.New()
	if err != nil {
		return nil, err
	}

	if arc.Type != "js" {
		return nil, errors.Errorf("expected bundle type 'js', got '%s'", arc.Type)
	}

	pgm, _, err := compiler.Compile(string(arc.Data), arc.FilenameURL.String(), "", "", true)
	if err != nil {
		return nil, err
	}

	initctx := NewInitContext(goja.New(), compiler, new(context.Context), arc.Filesystems, arc.PwdURL)

	env := arc.Env
	if env == nil {
		// Older archives (<=0.20.0) don't have an "env" property
		env = make(map[string]string)
	}
	for k, v := range rtOpts.Env {
		env[k] = v
	}

	bundle := &Bundle{
		Filename:        arc.FilenameURL,
		Source:          string(arc.Data),
		Program:         pgm,
		Options:         arc.Options,
		BaseInitContext: initctx,
		Env:             env,
	}
	if err := bundle.instantiate(bundle.BaseInitContext.runtime, bundle.BaseInitContext); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (b *Bundle) makeArchive() *lib.Archive {
	arc := &lib.Archive{
		Type:        "js",
		Filesystems: b.BaseInitContext.filesystems,
		Options:     b.Options,
		FilenameURL: b.Filename,
		Data:        []byte(b.Source),
		PwdURL:      b.BaseInitContext.pwd,
		Env:         make(map[string]string, len(b.Env)),
		K6Version:   consts.Version,
		Goos:        runtime.GOOS,
	}
	// Copy env so changes in the archive are not reflected in the source Bundle
	for k, v := range b.Env {
		arc.Env[k] = v
	}

	return arc
}

// Instantiate creates a new runtime from this bundle.
func (b *Bundle) Instantiate() (bi *BundleInstance, instErr error) {
	// Placeholder for a real context.
	ctxPtr := new(context.Context)

	// Instantiate the bundle into a new VM using a bound init context. This uses a context with a
	// runtime, but no state, to allow module-provided types to function within the init context.
	rt := goja.New()
	init := newBoundInitContext(b.BaseInitContext, ctxPtr, rt)
	if err := b.instantiate(rt, init); err != nil {
		return nil, err
	}

	// Grab the default function; type is already checked in NewBundle().
	exports := rt.Get("exports").ToObject(rt)
	def, ok := goja.AssertFunction(exports.Get("default"))
	if !ok || def == nil {
		panic("exported default is not a function")
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

	return &BundleInstance{
		Runtime: rt,
		Context: ctxPtr,
		Default: def,
	}, instErr
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.
func (b *Bundle) instantiate(rt *goja.Runtime, init *InitContext) error {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.NewRandSource())

	if _, err := rt.RunProgram(jslib.GetCoreJS()); err != nil {
		return err
	}

	exports := rt.NewObject()
	rt.Set("exports", exports)
	module := rt.NewObject()
	_ = module.Set("exports", exports)
	rt.Set("module", module)

	rt.Set("__ENV", b.Env)
	rt.Set("console", common.Bind(rt, newConsole(), init.ctxPtr))

	*init.ctxPtr = common.WithRuntime(context.Background(), rt)
	unbindInit := common.BindToGlobal(rt, common.Bind(rt, init, init.ctxPtr))
	if _, err := rt.RunProgram(b.Program); err != nil {
		return err
	}
	unbindInit()
	*init.ctxPtr = nil

	rt.SetRandSource(common.NewRandSource())

	return nil
}
