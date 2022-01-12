/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package loader

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"go.k6.io/k6/lib/fsext"
)

// ReadSource Reads a source file from any supported destination.
func ReadSource(
	logger logrus.FieldLogger, src, pwd string, filesystems map[string]afero.Fs, stdin io.Reader,
) (*SourceData, error) {
	if src == "-" {
		data, err := ioutil.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		// TODO: don't do it in this way ...
		err = afero.WriteFile(filesystems["file"].(fsext.CacheLayerGetter).GetCachingFs(), "/-", data, 0o644)
		if err != nil {
			return nil, fmt.Errorf("caching data read from -: %w", err)
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
		return Load(logger, filesystems, &url.URL{Scheme: "file", Path: filepath.ToSlash(srcLocalPath)}, src)
	}

	pwdURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(pwd)) + "/"}
	srcURL, err := Resolve(pwdURL, filepath.ToSlash(src))
	if err != nil {
		var noSchemeError noSchemeRemoteModuleResolutionError
		if errors.As(err, &noSchemeError) {
			// TODO maybe try to wrap the original error here as well, without butchering the message
			return nil, fmt.Errorf(nothingWorkedLoadedMsg, noSchemeError.moduleSpecifier, noSchemeError.err)
		}
		return nil, err
	}
	result, err := Load(logger, filesystems, srcURL, src)
	var noSchemeError noSchemeRemoteModuleResolutionError
	if errors.As(err, &noSchemeError) {
		// TODO maybe try to wrap the original error here as well, without butchering the message
		return nil, fmt.Errorf(nothingWorkedLoadedMsg, noSchemeError.moduleSpecifier, noSchemeError.err)
	}

	return result, err
}
