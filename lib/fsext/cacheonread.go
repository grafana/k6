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
