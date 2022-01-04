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
	"errors"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// ErrFileNeverOpenedBefore represent an error when file never opened before
var ErrFileNeverOpenedBefore = errors.New("file wasn't opened before")

// CacheOnReadFs is wrapper around afero.CacheOnReadFs with the ability to return the filesystem
// that is used as cache
type CacheOnReadFs struct {
	afero.Fs
	cache afero.Fs

	lock       *sync.Mutex
	openedOnly bool
	opened     map[string]bool
}

// OnlyOpenedEnabler enables the mode of FS that allows to open
// already opened files (e.g. serve from cache only)
type OnlyOpenedEnabler interface {
	AllowOnlyOpened()
}

// CacheLayerGetter provide a direct access to a cache layer
type CacheLayerGetter interface {
	GetCachingFs() afero.Fs
}

// NewCacheOnReadFs returns a new CacheOnReadFs
func NewCacheOnReadFs(base, layer afero.Fs, cacheTime time.Duration) afero.Fs {
	return &CacheOnReadFs{
		Fs:    afero.NewCacheOnReadFs(base, layer, cacheTime),
		cache: layer,

		lock:       &sync.Mutex{},
		openedOnly: false,
		opened:     make(map[string]bool),
	}
}

// GetCachingFs returns the afero.Fs being used for cache
func (c *CacheOnReadFs) GetCachingFs() afero.Fs {
	return c.cache
}

// AllowOnlyOpened enables the opened only mode of the CacheOnReadFs
func (c *CacheOnReadFs) AllowOnlyOpened() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.openedOnly = true
}

// Open opens file and track the history of opened files
// if CacheOnReadFs is in the opened only mode it should return
// an error if file wasn't open before
func (c *CacheOnReadFs) Open(name string) (afero.File, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.openedOnly {
		c.opened[name] = true
	} else if c.openedOnly && !c.opened[name] {
		return nil, ErrFileNeverOpenedBefore
	}

	return c.Fs.Open(name)
}
