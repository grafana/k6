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
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/loader"
)

type programWithSource struct {
	pgm    *goja.Program
	src    string
	module *goja.Object
}

const openCantBeUsedOutsideInitContextMsg = `The "open()" function is only available in the init stage ` +
	`(i.e. the global scope), see https://k6.io/docs/using-k6/test-life-cycle for more information`

// InitContext provides APIs for use in the init context.
//
// TODO: refactor most/all of this state away, use common.InitEnvironment instead
type InitContext struct {
	// Bound runtime; used to instantiate objects.
	runtime  *goja.Runtime
	compiler *compiler.Compiler

	// Pointer to a context that bridged modules are invoked with.
	ctxPtr *context.Context

	// Filesystem to load files and scripts from with the map key being the scheme
	filesystems map[string]afero.Fs
	pwd         *url.URL

	// Cache of loaded programs and files.
	programs map[string]programWithSource

	compatibilityMode lib.CompatibilityMode

	logger logrus.FieldLogger

	modules map[string]interface{}
}

// NewInitContext creates a new initcontext with the provided arguments
func NewInitContext(
	logger logrus.FieldLogger, rt *goja.Runtime, c *compiler.Compiler, compatMode lib.CompatibilityMode,
	ctxPtr *context.Context, filesystems map[string]afero.Fs, pwd *url.URL,
) *InitContext {
	return &InitContext{
		runtime:           rt,
		compiler:          c,
		ctxPtr:            ctxPtr,
		filesystems:       filesystems,
		pwd:               pwd,
		programs:          make(map[string]programWithSource),
		compatibilityMode: compatMode,
		logger:            logger,
		modules:           modules.GetJSModules(),
	}
}

func newBoundInitContext(base *InitContext, ctxPtr *context.Context, rt *goja.Runtime) *InitContext {
	// we don't copy the exports as otherwise they will be shared and we don't want this.
	// this means that all the files will be executed again but once again only once per compilation
	// of the main file.
	programs := make(map[string]programWithSource, len(base.programs))
	for key, program := range base.programs {
		programs[key] = programWithSource{
			src: program.src,
			pgm: program.pgm,
		}
	}
	return &InitContext{
		runtime: rt,
		ctxPtr:  ctxPtr,

		filesystems: base.filesystems,
		pwd:         base.pwd,
		compiler:    base.compiler,

		programs:          programs,
		compatibilityMode: base.compatibilityMode,
		logger:            base.logger,
		modules:           base.modules,
	}
}

// Require is called when a module/file needs to be loaded by a script
func (i *InitContext) Require(arg string) goja.Value {
	switch {
	case arg == "k6", strings.HasPrefix(arg, "k6/"):
		// Builtin or external modules ("k6", "k6/*", or "k6/x/*") are handled
		// specially, as they don't exist on the filesystem. This intentionally
		// shadows attempts to name your own modules this.
		v, err := i.requireModule(arg)
		if err != nil {
			common.Throw(i.runtime, err)
		}
		return v
	default:
		// Fall back to loading from the filesystem.
		v, err := i.requireFile(arg)
		if err != nil {
			common.Throw(i.runtime, err)
		}
		return v
	}
}

func (i *InitContext) requireModule(name string) (goja.Value, error) {
	mod, ok := i.modules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if perInstance, ok := mod.(modules.HasModuleInstancePerVU); ok {
		mod = perInstance.NewModuleInstancePerVU()
	}

	if withContext, ok := mod.(modules.HasWithContext); ok {
		// check that the original module was per instance as otherwise this will break and if not ... break
		if _, ok := i.modules[name].(modules.HasModuleInstancePerVU); !ok {
			// TODO better message ;)
			return nil, fmt.Errorf("module `%s` implement HasWithContext, but it does not implement HasModuleInstancePerVU which means that the context will be shared between VUs which is not what needs to happen. Please contact the developer of the module with this information.", name) //nolint:lll
		}
		withContext.WithContext(func() context.Context {
			return *i.ctxPtr
		})
		return i.runtime.ToValue(mod), nil
	}

	return i.runtime.ToValue(common.Bind(i.runtime, mod, i.ctxPtr)), nil
}

func (i *InitContext) requireFile(name string) (goja.Value, error) {
	// Resolve the file path, push the target directory as pwd to make relative imports work.
	pwd := i.pwd
	fileURL, err := loader.Resolve(pwd, name)
	if err != nil {
		return nil, err
	}

	// First, check if we have a cached program already.
	pgm, ok := i.programs[fileURL.String()]
	if !ok || pgm.module == nil {
		i.pwd = loader.Dir(fileURL)
		defer func() { i.pwd = pwd }()
		exports := i.runtime.NewObject()
		pgm.module = i.runtime.NewObject()
		_ = pgm.module.Set("exports", exports)

		if pgm.pgm == nil {
			// Load the sources; the loader takes care of remote loading, etc.
			data, err := loader.Load(i.logger, i.filesystems, fileURL, name)
			if err != nil {
				return goja.Undefined(), err
			}

			pgm.src = string(data.Data)

			// Compile the sources; this handles ES5 vs ES6 automatically.
			pgm.pgm, err = i.compileImport(pgm.src, data.URL.String())
			if err != nil {
				return goja.Undefined(), err
			}
		}

		i.programs[fileURL.String()] = pgm

		// Run the program.
		f, err := i.runtime.RunProgram(pgm.pgm)
		if err != nil {
			delete(i.programs, fileURL.String())
			return goja.Undefined(), err
		}
		if call, ok := goja.AssertFunction(f); ok {
			if _, err = call(exports, pgm.module, exports); err != nil {
				return nil, err
			}
		}
	}

	return pgm.module.Get("exports"), nil
}

func (i *InitContext) compileImport(src, filename string) (*goja.Program, error) {
	pgm, _, err := i.compiler.Compile(src, filename,
		"(function(module, exports){\n", "\n})\n", true, i.compatibilityMode)
	return pgm, err
}

// Open implements open() in the init context and will read and return the
// contents of a file. If the second argument is "b" it returns an ArrayBuffer
// instance, otherwise a string representation.
func (i *InitContext) Open(ctx context.Context, filename string, args ...string) (goja.Value, error) {
	if lib.GetState(ctx) != nil {
		return nil, errors.New(openCantBeUsedOutsideInitContextMsg)
	}

	if filename == "" {
		return nil, errors.New("open() can't be used with an empty filename")
	}

	// Here IsAbs should be enough but unfortunately it doesn't handle absolute paths starting from
	// the current drive on windows like `\users\noname\...`. Also it makes it more easy to test and
	// will probably be need for archive execution under windows if always consider '/...' as an
	// absolute path.
	if filename[0] != '/' && filename[0] != '\\' && !filepath.IsAbs(filename) {
		filename = filepath.Join(i.pwd.Path, filename)
	}
	filename = filepath.Clean(filename)
	fs := i.filesystems["file"]
	if filename[0:1] != afero.FilePathSeparator {
		filename = afero.FilePathSeparator + filename
	}
	// Workaround for https://github.com/spf13/afero/issues/201
	if isDir, err := afero.IsDir(fs, filename); err != nil {
		return nil, err
	} else if isDir {
		return nil, fmt.Errorf("open() can't be used with directories, path: %q", filename)
	}
	data, err := afero.ReadFile(fs, filename)
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && args[0] == "b" {
		ab := i.runtime.NewArrayBuffer(data)
		return i.runtime.ToValue(&ab), nil
	}
	return i.runtime.ToValue(string(data)), nil
}
