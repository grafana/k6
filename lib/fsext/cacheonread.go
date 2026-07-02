package fsext

import (
	"errors"
	"io/fs"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// ErrPathNeverRequestedBefore represent an error when path never opened/requested before
var ErrPathNeverRequestedBefore = errors.New("path never requested before")

// CacheOnReadFs is wrapper around afero.CacheOnReadFs with the ability to return the filesystem
// that is used as cache
type CacheOnReadFs struct {
	afero.Fs
	cache afero.Fs

	lock       *sync.Mutex
	cachedOnly bool
	cached     map[string]bool

	// populated marks paths whose first copyToLayer has finished; subsequent
	// Opens can then run concurrently through afero's cacheHit branch.
	//
	// Entries are never invalidated. k6 uses this wrapper read-only (see
	// internal/loader/filesystems.go), so Remove/RemoveAll/Rename/Chtimes/
	// Chmod - anything that triggers a fresh copyToLayer - would need to
	// clear the marker to avoid re-exposing the copyToLayer race.
	populated map[string]bool

	// fills holds a per-path mutex serializing the first
	// cacheMiss -> copyToLayer -> layer.Open in afero. Without it a second
	// reader can be classified as cacheHit on the still-being-copied layer
	// file and read empty/partial bytes.
	fills map[string]*sync.Mutex
}

// OnlyCachedEnabler enables the mode of FS that allows to open
// already opened files (e.g. serve from cache only)
type OnlyCachedEnabler interface {
	AllowOnlyCached()
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
		cachedOnly: false,
		cached:     make(map[string]bool),
		populated:  make(map[string]bool),
		fills:      make(map[string]*sync.Mutex),
	}
}

// GetCachingFs returns the afero.Fs being used for cache
func (c *CacheOnReadFs) GetCachingFs() afero.Fs {
	return c.cache
}

// AllowOnlyCached enables the cached only mode of the CacheOnReadFs
func (c *CacheOnReadFs) AllowOnlyCached() {
	c.lock.Lock()
	c.cachedOnly = true
	c.lock.Unlock()
}

// Open opens file and track the history of opened files
// if CacheOnReadFs is in the opened only mode it should return
// an error if file wasn't open before
func (c *CacheOnReadFs) Open(name string) (afero.File, error) {
	if err := c.checkOrRemember(name); err != nil {
		return nil, err
	}

	return c.openSerialized(name, func() (afero.File, error) {
		return c.Fs.Open(name)
	})
}

// OpenFile shares openSerialized with Open so a first cacheMiss can't race
// with a concurrent reader. Read-only flags only: a write/truncate flag
// mutates the layer while populated stays true, letting concurrent fast-path
// Opens read torn or empty data (see the populated field comment).
func (c *CacheOnReadFs) OpenFile(name string, flag int, perm fs.FileMode) (afero.File, error) {
	if err := c.checkOrRemember(name); err != nil {
		return nil, err
	}

	return c.openSerialized(name, func() (afero.File, error) {
		return c.Fs.OpenFile(name, flag, perm)
	})
}

// openSerialized invokes openFn while ensuring that, for a given name, the
// first successful open completes before any other goroutine is allowed into
// the same open path. Once a name is marked populated, openFn is invoked with
// no extra locking.
func (c *CacheOnReadFs) openSerialized(name string, openFn func() (afero.File, error)) (afero.File, error) {
	c.lock.Lock()
	if c.populated[name] {
		c.lock.Unlock()
		return openFn()
	}
	fl, ok := c.fills[name]
	if !ok {
		fl = &sync.Mutex{}
		c.fills[name] = fl
	}
	c.lock.Unlock()

	fl.Lock()

	// Filler already finished: drop fl so queued waiters fan out in parallel
	// through afero's cacheHit branch, and skip the redundant bookkeeping.
	c.lock.Lock()
	alreadyPopulated := c.populated[name]
	c.lock.Unlock()
	if alreadyPopulated {
		fl.Unlock()
		return openFn()
	}

	defer fl.Unlock()

	f, err := openFn()
	if err != nil {
		return nil, err
	}

	c.lock.Lock()
	c.populated[name] = true
	delete(c.fills, name)
	c.lock.Unlock()

	return f, nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
// if CacheOnReadFs is in the opened only mode it should return
// an error if path wasn't open before
//
// Not routed through openSerialized: a concurrent Stat during another
// goroutine's first Open can see the layer file mid copyToLayer and report
// Size=0. Safe today because k6's Stat callers only read presence/IsDir.
// Route through openSerialized if a caller starts consuming Size/ModTime/Mode.
func (c *CacheOnReadFs) Stat(path string) (fs.FileInfo, error) {
	if err := c.checkOrRemember(path); err != nil {
		return nil, err
	}

	return c.Fs.Stat(path)
}

func (c *CacheOnReadFs) checkOrRemember(path string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.cachedOnly {
		c.cached[path] = true
	} else if !c.cached[path] {
		return ErrPathNeverRequestedBefore
	}

	return nil
}
