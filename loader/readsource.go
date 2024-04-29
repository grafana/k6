package loader

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib/fsext"
)

// ReadSource Reads a source file from any supported destination.
func ReadSource(
	logger logrus.FieldLogger, src, pwd string, filesystems map[string]fsext.Fs, stdin io.Reader,
) (*SourceData, error) {
	// 'ToSlash' is here as URL only use '/' as separators, but on Windows paths use '\'
	pwdURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(pwd)) + "/"}
	if src == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		// TODO: don't do it in this way ...
		//nolint:forcetypeassert
		err = fsext.WriteFile(filesystems["file"].(fsext.CacheLayerGetter).GetCachingFs(), "/-", data, 0o644)
		if err != nil {
			return nil, fmt.Errorf("caching data read from -: %w", err)
		}
		return &SourceData{URL: &url.URL{Path: "/-", Scheme: "file"}, Data: data, PWD: pwdURL}, err
	}
	var srcLocalPath string
	if filepath.IsAbs(src) {
		srcLocalPath = src
	} else {
		srcLocalPath = filepath.Join(pwd, src)
	}
	// All paths should start with a / in all fses. This is mostly for windows where it will start
	// with a volume name : C:\something.js
	srcLocalPath = filepath.Clean(fsext.FilePathSeparator + srcLocalPath)
	if ok, _ := fsext.Exists(filesystems["file"], srcLocalPath); ok {
		// there is file on the local disk ... lets use it :)
		return Load(logger, filesystems, &url.URL{Scheme: "file", Path: filepath.ToSlash(srcLocalPath)}, src)
	}

	srcURL, err := Resolve(pwdURL, filepath.ToSlash(src))
	if err != nil {
		var unresolvedError unresolvableURLError
		if errors.As(err, &unresolvedError) {
			//nolint:stylecheck
			return nil, fmt.Errorf(fileSchemeCouldntBeLoadedMsg, (string)(unresolvedError))
		}
		return nil, err
	}
	return Load(logger, filesystems, srcURL, src)
}
