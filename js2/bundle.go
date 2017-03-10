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
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/compiler"
	"github.com/loadimpact/k6/lib"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"path/filepath"
	"reflect"
)

func compile(src *lib.SourceData) (*goja.Program, error) {
	code, _, err := compiler.Transform(string(src.Data), src.Filename)
	if err != nil {
		return nil, err
	}

	return goja.Compile(src.Filename, code, true)
}

// A Bundle is a self-contained bundle of scripts and resources.
// You can use this to produce identical VMs.
type Bundle struct {
	Filename    string
	Program     *goja.Program
	InitContext *InitContext
}

// Creates a new bundle from a source file and a filesystem.
func NewBundle(src *lib.SourceData, fs afero.Fs) (*Bundle, error) {
	// Compile the main program, use the script's dir as initial pwd.
	pgm, err := compile(src)
	if err != nil {
		return nil, err
	}

	// We want to eliminate disk access at runtime, so we set up a memory mapped cache that's
	// written every time something is read from the real filesystem. This cache is then used for
	// successive spawns to read from (they have no access to the real disk).
	mirrorFS := afero.NewMemMapFs()
	cachedFS := afero.NewCacheOnReadFs(fs, mirrorFS, 0)

	// Make a bundle, instantiate it into a throwaway VM to populate caches.
	bundle := Bundle{
		Filename: src.Filename,
		Program:  pgm,
		InitContext: &InitContext{
			Fs:      cachedFS,
			Pwd:     filepath.Dir(src.Filename),
			Modules: make(map[string]*goja.Program),
		},
	}
	rt, err := bundle.Instantiate()
	if err != nil {
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

	// Swap out the init context's filesystem for the in-memory cache.
	bundle.InitContext.Fs = mirrorFS

	return &bundle, nil
}

// Instantiates a new runtime from this bundle.
func (b *Bundle) Instantiate() (*goja.Runtime, error) {
	rt := goja.New()
	rt.SetFieldNameMapper(FieldNameMapper{})
	rt.Set("exports", rt.NewObject())

	rt.SetRandSource(DefaultRandSource)
	unbindInit := BindToGlobal(rt, b.InitContext)
	if _, err := rt.RunProgram(b.Program); err != nil {
		return nil, err
	}
	unbindInit()
	rt.SetRandSource(NewRandSource())

	return rt, nil
}
