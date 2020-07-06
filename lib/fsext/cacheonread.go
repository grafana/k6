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

package fsext

import (
	"time"

	"github.com/spf13/afero"
)

// CacheOnReadFs is wrapper around afero.CacheOnReadFs with the ability to return the filesystem
// that is used as cache
type CacheOnReadFs struct {
	afero.Fs
	cache afero.Fs
}

// NewCacheOnReadFs returns a new CacheOnReadFs
func NewCacheOnReadFs(base, layer afero.Fs, cacheTime time.Duration) afero.Fs {
	return CacheOnReadFs{
		Fs:    afero.NewCacheOnReadFs(base, layer, cacheTime),
		cache: layer,
	}
}

// GetCachingFs returns the afero.Fs being used for cache
func (c CacheOnReadFs) GetCachingFs() afero.Fs {
	return c.cache
}
