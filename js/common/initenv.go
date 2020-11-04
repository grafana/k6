/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package common

import (
	"net/url"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// InitEnvironment contains properties that can be accessed by Go code executed
// in the k6 init context. It can be accessed by calling common.GetInitEnv().
type InitEnvironment struct {
	Logger      logrus.FieldLogger
	FileSystems map[string]afero.Fs
	CWD         *url.URL
	// TODO: add RuntimeOptions and other properties, goja sources, etc.
	// ideally, we should leave this as the only data structure necessary for
	// executing the init context for all JS modules
}

// GetAbsFilePath should be used to access the FileSystems, since afero has a
// bug when opening files with relative paths - it caches them from the FS root,
// not the current working directory... So, if necessary, this method will
// transform any relative paths into absolute ones, using the CWD.
//
// TODO: refactor? It was copied from
// https://github.com/loadimpact/k6/blob/c51095ad7304bdd1e82cdb33c91abc331533b886/js/initcontext.go#L211-L222
func (ie *InitEnvironment) GetAbsFilePath(filename string) string {
	// Here IsAbs should be enough but unfortunately it doesn't handle absolute paths starting from
	// the current drive on windows like `\users\noname\...`. Also it makes it more easy to test and
	// will probably be need for archive execution under windows if always consider '/...' as an
	// absolute path.
	if filename[0] != '/' && filename[0] != '\\' && !filepath.IsAbs(filename) {
		filename = filepath.Join(ie.CWD.Path, filename)
	}
	filename = filepath.Clean(filename)
	if filename[0:1] != afero.FilePathSeparator {
		filename = afero.FilePathSeparator + filename
	}
	return filename
}
