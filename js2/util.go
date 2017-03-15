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
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
)

// Shortcut to creating a bundle and instantiating a bundle right away.
// Useful for writing tests, or when you just don't care about the bundle.
func QuickInstance(src *lib.SourceData, fs afero.Fs) (*goja.Runtime, error) {
	b, err := NewBundle(src, fs)
	if err != nil {
		return nil, err
	}
	return b.Instantiate()
}
