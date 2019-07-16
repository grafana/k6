package loader

import (
	"io"
	"io/ioutil"
	"net/url"
	"path/filepath"

	"github.com/loadimpact/k6/lib/fsext"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// ReadSource Reads a source file from any supported destination.
func ReadSource(src, pwd string, filesystems map[string]afero.Fs, stdin io.Reader) (*SourceData, error) {
	if src == "-" {
		data, err := ioutil.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		// TODO: don't do it in this way ...
		err = afero.WriteFile(filesystems["file"].(fsext.CacheOnReadFs).GetCachingFs(), "/-", data, 0644)
		if err != nil {
			return nil, errors.Wrap(err, "caching data read from -")
		}
		return &SourceData{URL: &url.URL{Path: "/-", Scheme: "file"}, Data: data}, err
	}
	var srcLocalPath string
	if filepath.IsAbs(src) {
		srcLocalPath = src
	} else {
		srcLocalPath = filepath.Join(pwd, src)
	}
	// All paths should start with a / in all fses. This is mostly for windows where it will start
	// with a volume name : C:\something.js
	srcLocalPath = filepath.Clean(afero.FilePathSeparator + srcLocalPath)
	if ok, _ := afero.Exists(filesystems["file"], srcLocalPath); ok {
		// there is file on the local disk ... lets use it :)
		return Load(filesystems, &url.URL{Scheme: "file", Path: filepath.ToSlash(srcLocalPath)}, src)
	}

	pwdURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(pwd)) + "/"}
	srcURL, err := Resolve(pwdURL, filepath.ToSlash(src))
	if err != nil {
		return nil, err
	}
	return Load(filesystems, srcURL, src)
}
