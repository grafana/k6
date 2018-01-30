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
	"os"
	"strings"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/compiler"
	jslib "github.com/loadimpact/k6/js/lib"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	Filename string
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

func collectEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := strings.IndexRune(kv, '='); idx != -1 {
			env[kv[:idx]] = kv[idx+1:]
		} else {
			env[kv] = ""
		}
	}
	return env
}

// Creates a new bundle from a source file and a filesystem.
func NewBundle(src *lib.SourceData, fs afero.Fs) (*Bundle, error) {
	// Compile sources, both ES5 and ES6 are supported.
	code := string(src.Data)
	pgm, _, err := compiler.Compile(code, src.Filename, "", "", true)
	if err != nil {
		return nil, err
	}

	// We want to eliminate disk access at runtime, so we set up a memory mapped cache that's
	// written every time something is read from the real filesystem. This cache is then used for
	// successive spawns to read from (they have no access to the real disk).
	// CURRENTLY BROKEN: https://github.com/spf13/afero/issues/115
	// mirrorFS := afero.NewMemMapFs()
	// cachedFS := afero.NewCacheOnReadFs(fs, mirrorFS, 0)

	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	rt := goja.New()
	bundle := Bundle{
		Filename:        src.Filename,
		Source:          code,
		Program:         pgm,
		BaseInitContext: NewInitContext(rt, new(context.Context), fs, loader.Dir(src.Filename)),
		Env:             collectEnv(),
	}
	if err := bundle.instantiate(rt, bundle.BaseInitContext); err != nil {
		return nil, err
	}

	// Validate exports.
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

	// Extract exported options.
	optV := exports.Get("options")
	if optV != nil && !goja.IsNull(optV) && !goja.IsUndefined(optV) {
		optdata, err := json.Marshal(optV.Export())
		if err != nil {
			return nil, err
		}
		var f interface{}
		if err := json.Unmarshal(optdata, &f); err != nil {
			return nil, err
		}
		switch f.(type) {
		case []interface{}:
			var tempOptions []lib.Options
			if err := json.Unmarshal(optdata, &tempOptions); err != nil {
				return nil, err
			}

			// merge things, there's probably an easier way to do this but it works at the moment
			mergedThresholds := make(map[string]stats.Thresholds)
			for _, options := range tempOptions {
				for k := range options.Thresholds {
					runtime := options.Thresholds[k].Runtime
					var tempThresholds []*stats.Threshold
					tempThresholds = append(mergedThresholds[k].Thresholds, options.Thresholds[k].Thresholds...)
					mergedThresholds[k] = stats.Thresholds{Runtime: runtime, Thresholds: tempThresholds}
				}
			}
			// take the last options from the array and replace the thresholds with merged thresholds
			bundle.Options = tempOptions[len(tempOptions)-1]
			bundle.Options.Thresholds = mergedThresholds
		case map[string]interface{}:
			if err := json.Unmarshal(optdata, &bundle.Options); err != nil {
				return nil, err
			}
		}
	}

	// Swap out the init context's filesystem for the in-memory cache.
	// bundle.InitContext.fs = mirrorFS

	return &bundle, nil
}

func NewBundleFromArchive(arc *lib.Archive) (*Bundle, error) {
	if arc.Type != "js" {
		return nil, errors.Errorf("expected bundle type 'js', got '%s'", arc.Type)
	}

	pgm, _, err := compiler.Compile(string(arc.Data), arc.Filename, "", "", true)
	if err != nil {
		return nil, err
	}

	initctx := NewInitContext(goja.New(), new(context.Context), nil, arc.Pwd)
	for filename, data := range arc.Scripts {
		src := string(data)
		pgm, err := initctx.compileImport(src, filename)
		if err != nil {
			return nil, err
		}
		initctx.programs[filename] = programWithSource{pgm, src}
	}
	initctx.files = arc.Files

	return &Bundle{
		Filename:        arc.Filename,
		Source:          string(arc.Data),
		Program:         pgm,
		Options:         arc.Options,
		BaseInitContext: initctx,
		Env:             collectEnv(),
	}, nil
}

func (b *Bundle) MakeArchive() *lib.Archive {
	arc := &lib.Archive{
		Type:     "js",
		Options:  b.Options,
		Filename: b.Filename,
		Data:     []byte(b.Source),
		Pwd:      b.BaseInitContext.pwd,
	}

	arc.Scripts = make(map[string][]byte, len(b.BaseInitContext.programs))
	for name, pgm := range b.BaseInitContext.programs {
		arc.Scripts[name] = []byte(pgm.src)
	}
	arc.Files = b.BaseInitContext.files

	return arc
}

// Instantiates a new runtime from this bundle.
func (b *Bundle) Instantiate() (*BundleInstance, error) {
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

	return &BundleInstance{
		Runtime: rt,
		Context: ctxPtr,
		Default: def,
	}, nil
}

// Instantiates the bundle into an existing runtime. Not public because it also messes with a bunch
// of other things, will potentially thrash data and makes a mess in it if the operation fails.
func (b *Bundle) instantiate(rt *goja.Runtime, init *InitContext) error {
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.SetRandSource(common.DefaultRandSource)

	if _, err := rt.RunProgram(jslib.CoreJS); err != nil {
		return err
	}

	exports := rt.NewObject()
	rt.Set("exports", exports)
	module := rt.NewObject()
	_ = module.Set("exports", exports)
	rt.Set("module", module)

	rt.Set("__ENV", b.Env)

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
