package js

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/loader"
)

type programWithSource struct {
	pgm     *goja.Program
	src     string
	module  *goja.Object
	exports *goja.Object
}

const openCantBeUsedOutsideInitContextMsg = `The "open()" function is only available in the init stage ` +
	`(i.e. the global scope), see https://k6.io/docs/using-k6/test-life-cycle for more information`

// InitContext provides APIs for use in the init context.
//
// TODO: refactor most/all of this state away, use common.InitEnvironment instead
type InitContext struct {
	// Bound runtime; used to instantiate objects.
	compiler *compiler.Compiler

	moduleVUImpl *moduleVUImpl

	// Filesystem to load files and scripts from with the map key being the scheme
	filesystems map[string]afero.Fs
	pwd         *url.URL

	// Cache of loaded programs and files.
	programs map[string]programWithSource
	// merge this and the above
	k6ModulesCache map[string]goja.Value

	compatibilityMode lib.CompatibilityMode

	logger logrus.FieldLogger

	moduleRegistry map[string]interface{}
}

// NewInitContext creates a new initcontext with the provided arguments
func NewInitContext(
	logger logrus.FieldLogger, rt *goja.Runtime, c *compiler.Compiler, compatMode lib.CompatibilityMode,
	filesystems map[string]afero.Fs, pwd *url.URL,
) *InitContext {
	return &InitContext{
		compiler:          c,
		filesystems:       filesystems,
		pwd:               pwd,
		programs:          make(map[string]programWithSource),
		compatibilityMode: compatMode,
		logger:            logger,
		moduleRegistry:    getJSModules(),
		k6ModulesCache:    make(map[string]goja.Value),
		moduleVUImpl: &moduleVUImpl{
			// TODO: pass a real context as we did for https://github.com/grafana/k6/pull/2800,
			// also see https://github.com/grafana/k6/issues/2804
			ctx:     context.Background(),
			runtime: rt,
		},
	}
}

func newBoundInitContext(base *InitContext, vuImpl *moduleVUImpl) *InitContext {
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
		filesystems: base.filesystems,
		pwd:         base.pwd,
		compiler:    base.compiler,

		programs:          programs,
		compatibilityMode: base.compatibilityMode,
		k6ModulesCache:    make(map[string]goja.Value),
		logger:            base.logger,
		moduleRegistry:    base.moduleRegistry,
		moduleVUImpl:      vuImpl,
	}
}

// Require is called when a module/file needs to be loaded by a script
func (i *InitContext) Require(arg string) (export goja.Value) {
	switch {
	case arg == "k6", strings.HasPrefix(arg, "k6/"):
		var ok bool
		if export, ok = i.k6ModulesCache[arg]; ok {
			return export
		}
		defer func() { i.k6ModulesCache[arg] = export }()
		// Builtin or external modules ("k6", "k6/*", or "k6/x/*") are handled
		// specially, as they don't exist on the filesystem. This intentionally
		// shadows attempts to name your own modules this.
		v, err := i.requireModule(arg)
		if err != nil {
			common.Throw(i.moduleVUImpl.runtime, err)
		}
		return v
	default:
		// Fall back to loading from the filesystem.
		v, err := i.requireFile(arg)
		if err != nil {
			common.Throw(i.moduleVUImpl.runtime, err)
		}
		return v
	}
}

func toESModuleExports(exp modules.Exports) interface{} {
	if exp.Named == nil {
		return exp.Default
	}
	if exp.Default == nil {
		return exp.Named
	}

	result := make(map[string]interface{}, len(exp.Named)+2)

	for k, v := range exp.Named {
		result[k] = v
	}
	// Maybe check that those weren't set
	result["default"] = exp.Default
	// this so babel works with the `default` when it transpiles from ESM to commonjs.
	// This should probably be removed once we have support for ESM directly. So that require doesn't get support for
	// that while ESM has.
	result["__esModule"] = true

	return result
}

func (i *InitContext) requireModule(name string) (goja.Value, error) {
	mod, ok := i.moduleRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}
	if m, ok := mod.(modules.Module); ok {
		instance := m.NewModuleInstance(i.moduleVUImpl)
		return i.moduleVUImpl.runtime.ToValue(toESModuleExports(instance.Exports())), nil
	}

	return i.moduleVUImpl.runtime.ToValue(mod), nil
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
		if filepath.IsAbs(name) && runtime.GOOS == "windows" {
			i.logger.Warnf("'%s' was imported with an absolute path - this won't be cross-platform and won't work if"+
				" you move the script between machines or run it with `k6 cloud`; if absolute paths are required,"+
				" import them with the `file://` schema for slightly better compatibility",
				name)
		}
		i.pwd = loader.Dir(fileURL)
		defer func() { i.pwd = pwd }()
		exports := i.moduleVUImpl.runtime.NewObject()
		pgm.module = i.moduleVUImpl.runtime.NewObject()
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
		f, err := i.moduleVUImpl.runtime.RunProgram(pgm.pgm)
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
	pgm, _, err := i.compiler.Compile(src, filename, false)
	return pgm, err
}

// Open implements open() in the init context and will read and return the
// contents of a file. If the second argument is "b" it returns an ArrayBuffer
// instance, otherwise a string representation.
func (i *InitContext) Open(filename string, args ...string) (goja.Value, error) {
	if i.moduleVUImpl.State() != nil {
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

	data, err := readFile(fs, filename)
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && args[0] == "b" {
		ab := i.moduleVUImpl.runtime.NewArrayBuffer(data)
		return i.moduleVUImpl.runtime.ToValue(&ab), nil
	}
	return i.moduleVUImpl.runtime.ToValue(string(data)), nil
}

func readFile(fileSystem afero.Fs, filename string) (data []byte, err error) {
	defer func() {
		if errors.Is(err, fsext.ErrPathNeverRequestedBefore) {
			// loading different files per VU is not supported, so all files should are going
			// to be used inside the scenario should be opened during the init step (without any conditions)
			err = fmt.Errorf(
				"open() can't be used with files that weren't previously opened during initialization (__VU==0), path: %q",
				filename,
			)
		}
	}()

	// Workaround for https://github.com/spf13/afero/issues/201
	if isDir, err := afero.IsDir(fileSystem, filename); err != nil {
		return nil, err
	} else if isDir {
		return nil, fmt.Errorf("open() can't be used with directories, path: %q", filename)
	}

	return afero.ReadFile(fileSystem, filename)
}

// allowOnlyOpenedFiles enables seen only files
func (i *InitContext) allowOnlyOpenedFiles() {
	fs := i.filesystems["file"]

	alreadyOpenedFS, ok := fs.(fsext.OnlyCachedEnabler)
	if !ok {
		return
	}

	alreadyOpenedFS.AllowOnlyCached()
}
