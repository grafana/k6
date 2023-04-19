package common

import (
	"net/url"
	"path/filepath"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
)

// InitEnvironment contains properties that can be accessed by Go code executed
// in the k6 init context. It can be accessed by calling common.GetInitEnv().
type InitEnvironment struct {
	*lib.TestPreInitState
	FileSystems map[string]fsext.Fs
	CWD         *url.URL
	// TODO: get rid of this type altogether? we won't need it if we figure out
	// how to handle .tar archive vs regular JS script differences in FileSystems
}

// GetAbsFilePath should be used to access the FileSystems, since afero has a
// bug when opening files with relative paths - it caches them from the FS root,
// not the current working directory... So, if necessary, this method will
// transform any relative paths into absolute ones, using the CWD.
//
// TODO: refactor? It was copied from
// https://github.com/k6io/k6/blob/c51095ad7304bdd1e82cdb33c91abc331533b886/js/initcontext.go#L211-L222
func (ie *InitEnvironment) GetAbsFilePath(filename string) string {
	// Here IsAbs should be enough but unfortunately it doesn't handle absolute paths starting from
	// the current drive on windows like `\users\noname\...`. Also it makes it more easy to test and
	// will probably be need for archive execution under windows if always consider '/...' as an
	// absolute path.
	if filename[0] != '/' && filename[0] != '\\' && !filepath.IsAbs(filename) {
		filename = filepath.Join(ie.CWD.Path, filename)
	}
	filename = filepath.Clean(filename)
	if filename[0:1] != fsext.FilePathSeparator {
		filename = fsext.FilePathSeparator + filename
	}
	return filename
}
