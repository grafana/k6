package k6provider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Pruner prunes binaries suing a LRU policy to enforce a limit
// defined in a high-water-mark.
type Pruner struct {
	pruneLock     sync.Mutex
	dirLock       *dirLock
	dir           string
	hwm           int64
	pruneInterval time.Duration
	lastPrune     time.Time
}

type pruneTarget struct {
	path      string
	size      int64
	timestamp time.Time
}

// NewPruner creates a [Pruner] given its high-water-mark limit, and the
// prune interval
func NewPruner(dir string, hwm int64, pruneInterval time.Duration) *Pruner {
	return &Pruner{
		dirLock:       newDirLock(dir),
		dir:           dir,
		hwm:           hwm,
		pruneInterval: pruneInterval,
	}
}

// Touch update access time because reading the file not always updates it
func (p *Pruner) Touch(binPath string) {
	if p.hwm > 0 {
		p.pruneLock.Lock()
		defer p.pruneLock.Unlock()
		_ = os.Chtimes(binPath, time.Now(), time.Now())
	}
}

// Prune the cache of least recently used files
func (p *Pruner) Prune() error {
	if p.hwm == 0 {
		return nil
	}

	// if a lock exists, another prune is in progress
	if !p.pruneLock.TryLock() {
		return nil
	}
	defer p.pruneLock.Unlock()

	if time.Since(p.lastPrune) < p.pruneInterval {
		return nil
	}
	p.lastPrune = time.Now()

	// prevent concurrent prune to the directory
	err := p.dirLock.tryLock()
	if err != nil {
		// is locked, another pruner must be running (maybe another process)
		if errors.Is(err, errLocked) {
			return nil
		}
		return fmt.Errorf("%w: %w", ErrPruningCache, err)
	}
	defer func() {
		_ = p.dirLock.unlock()
	}()

	binaries, err := os.ReadDir(p.dir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPruningCache, err)
	}

	errs := []error{ErrPruningCache}
	cacheSize := int64(0)
	pruneTargets := []pruneTarget{}
	for _, binDir := range binaries {
		// skip any spurious file, each binary is in a directory
		if !binDir.IsDir() {
			continue
		}

		binPath := filepath.Join(p.dir, binDir.Name(), k6Binary)
		binInfo, err := os.Stat(binPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cacheSize += binInfo.Size()
		pruneTargets = append(
			pruneTargets,
			pruneTarget{
				path:      filepath.Dir(binPath), // we are going to prune the directory
				size:      binInfo.Size(),
				timestamp: binInfo.ModTime(),
			})
	}

	if cacheSize <= p.hwm {
		return nil
	}

	sort.Slice(pruneTargets, func(i, j int) bool {
		return pruneTargets[i].timestamp.Before(pruneTargets[j].timestamp)
	})

	for _, target := range pruneTargets {
		if err := os.RemoveAll(target.path); err != nil {
			errs = append(errs, err)
			continue
		}

		cacheSize -= target.size
		if cacheSize <= p.hwm {
			return nil
		}
	}

	return fmt.Errorf("%w cache could not be pruned", errors.Join(errs...))
}
