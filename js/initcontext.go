package js

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/loader"
)

const cantBeUsedOutsideInitContextMsg = `the "%s" function is only available in the init stage ` +
	`(i.e. the global scope), see https://k6.io/docs/using-k6/test-life-cycle for more information`

// openImpl implements openImpl() in the init context and will read and return the
// contents of a file. If the second argument is "b" it returns an ArrayBuffer
// instance, otherwise a string representation.
func openImpl(rt *goja.Runtime, fs afero.Fs, basePWD *url.URL, filename string, args ...string) (goja.Value, error) {
	// Here IsAbs should be enough but unfortunately it doesn't handle absolute paths starting from
	// the current drive on windows like `\users\noname\...`. Also it makes it more easy to test and
	// will probably be need for archive execution under windows if always consider '/...' as an
	// absolute path.
	if filename[0] != '/' && filename[0] != '\\' && !filepath.IsAbs(filename) {
		filename = filepath.Join(basePWD.Path, filename)
	}
	filename = filepath.Clean(filename)

	if filename[0:1] != afero.FilePathSeparator {
		filename = afero.FilePathSeparator + filename
	}

	data, err := readFile(fs, filename)
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && args[0] == "b" {
		ab := rt.NewArrayBuffer(data)
		return rt.ToValue(&ab), nil
	}
	return rt.ToValue(string(data)), nil
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
func allowOnlyOpenedFiles(fs afero.Fs) {
	alreadyOpenedFS, ok := fs.(fsext.OnlyCachedEnabler)
	if !ok {
		return
	}

	alreadyOpenedFS.AllowOnlyCached()
}

type requireImpl struct {
	vu      modules.VU
	modules *moduleSystem
	pwd     *url.URL
}

func (r *requireImpl) require(specifier string) (*goja.Object, error) {
	// TODO remove this in the future when we address https://github.com/grafana/k6/issues/2674
	// This is currently needed as each time require is called we need to record it's new pwd
	// to be used if a require *or* open is used within the file as they are relative to the
	// latest call to require.
	// This is *not* the actual require behaviour defined in commonJS as it is actually always relative
	// to the file it is in. This is unlikely to be an issue but this code is here to keep backwards
	// compatibility *for now*.
	// With native ESM this won't even be possible as `require` might not be called - instead an import
	// might be used in which case we won't be able to be doing this hack. In that case we either will
	// need some goja specific helper or to use stack traces as goja_nodejs does.
	currentPWD := r.pwd
	if specifier != "k6" && !strings.HasPrefix(specifier, "k6/") {
		defer func() {
			r.pwd = currentPWD
		}()
		// In theory we can give that downwards, but this makes the code more tightly coupled
		// plus as explained above this will be removed in the future so the code reflects more
		// closely what will be needed then
		fileURL, err := loader.Resolve(r.pwd, specifier)
		if err != nil {
			return nil, err
		}
		r.pwd = loader.Dir(fileURL)
	}

	if r.vu.State() != nil { // fix
		return nil, fmt.Errorf(cantBeUsedOutsideInitContextMsg, "require")
	}
	if specifier == "" {
		return nil, errors.New("require() can't be used with an empty specifier")
	}

	return r.modules.Require(currentPWD, specifier)
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
