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

package js2

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/compiler"
	"github.com/loadimpact/k6/js2/common"
	"github.com/loadimpact/k6/lib"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical BundleInstance objects.
type Bundle struct {
	Filename string
	Program  *goja.Program
	Options  lib.Options

	BaseInitContext *InitContext
}

// A BundleInstance is a self-contained instance of a Bundle.
type BundleInstance struct {
	Runtime *goja.Runtime
	Context *context.Context
	Default goja.Callable
}

// Creates a new bundle from a source file and a filesystem.
func NewBundle(src *lib.SourceData, fs afero.Fs) (*Bundle, error) {
	// Compile the main program.
	code, _, err := compiler.Transform(string(src.Data), src.Filename)
	if err != nil {
		return nil, err
	}
	pgm, err := goja.Compile(src.Filename, code, true)
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
		Program:         pgm,
		BaseInitContext: NewInitContext(rt, new(context.Context), fs, filepath.Dir(src.Filename)),
	}
	if err := bundle.instantiate(rt, bundle.BaseInitContext); err != nil {
		return nil, err
	}

	// Validate exports.
	exports := rt.Get("exports").ToObject(rt)
	if exports == nil {
		return nil, errors.New("exports must be an object")
	}

	// Validate the default function.
	def := exports.Get("default")
	if def == nil || goja.IsNull(def) || goja.IsUndefined(def) {
		return nil, errors.New("script must export a default function")
	}
	if def.ExportType().Kind() != reflect.Func {
		return nil, errors.New("default export must be a function")
	}

	// Extract exported options.
	optV := exports.Get("options")
	if optV != nil && !goja.IsNull(optV) && !goja.IsUndefined(optV) {
		optdata, err := json.Marshal(optV.Export())
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(optdata, &bundle.Options); err != nil {
			return nil, err
		}
	}

	// Swap out the init context's filesystem for the in-memory cache.
	// bundle.InitContext.fs = mirrorFS

	return &bundle, nil
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
	def, _ := goja.AssertFunction(exports.Get("default"))

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
	rt.Set("exports", rt.NewObject())

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
