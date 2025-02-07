package js

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/lib/fsext"
)

const cantBeUsedOutsideInitContextMsg = `the "%s" function is only available in the init stage ` +
	`(i.e. the global scope), see https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/ for more information`

// openImpl implements openImpl() in the init context and will read and return the
// contents of a file. If the second argument is "b" it returns an ArrayBuffer
// instance, otherwise a string representation.
func openImpl(rt *sobek.Runtime, fs fsext.Fs, basePWD *url.URL, filename string, args ...string) (sobek.Value, error) {
	// Strip file scheme if available as we should support only this scheme
	filename = strings.TrimPrefix(filename, "file://")
	data, err := readFile(fs, fsext.Abs(basePWD.Path, filename))
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && args[0] == "b" {
		ab := rt.NewArrayBuffer(data)
		return rt.ToValue(&ab), nil
	}
	return rt.ToValue(string(data)), nil
}

func readFile(fileSystem fsext.Fs, filename string) (data []byte, err error) {
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

	// Workaround for https://github.com/spf13/fsext/issues/201
	if isDir, err := fsext.IsDir(fileSystem, filename); err != nil {
		return nil, err
	} else if isDir {
		return nil, fmt.Errorf("open() can't be used with directories, path: %q", filename)
	}

	return fsext.ReadFile(fileSystem, filename)
}

// allowOnlyOpenedFiles enables seen only files
func allowOnlyOpenedFiles(fs fsext.Fs) {
	alreadyOpenedFS, ok := fs.(fsext.OnlyCachedEnabler)
	if !ok {
		return
	}

	alreadyOpenedFS.AllowOnlyCached()
}

func generateSourceMapLoader(logger logrus.FieldLogger, filesystems map[string]fsext.Fs,
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
