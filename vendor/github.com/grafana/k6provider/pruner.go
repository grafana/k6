package k6provider

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Pruner prunes binaries using a LRU policy to enforce a limit
// defined in a high-water-mark.
type Pruner struct {
	pruneLock     sync.Mutex
	dirLock       *dirLock
	dir           string
	hwm           int64
	pruneInterval time.Duration
	lastPrune     time.Time
	logger        *slog.Logger
}

type pruneTarget struct {
	path      string
	size      int64
	timestamp time.Time
}

// NewPruner creates a [Pruner] given its high-water-mark limit, and the
// prune interval
func NewPruner(dir string, hwm int64, pruneInterval time.Duration) *Pruner {
	return NewPrunerWithLogger(dir, hwm, pruneInterval, nil)
}

// NewPrunerWithLogger creates a [Pruner] with the given logger.
// If logger is nil, a discard logger is used.
func NewPrunerWithLogger(dir string, hwm int64, pruneInterval time.Duration, logger *slog.Logger) *Pruner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Pruner{
		dirLock:       newDirLock(dir),
		dir:           dir,
		hwm:           hwm,
		pruneInterval: pruneInterval,
		logger:        logger,
	}
}

// Touch update access time because reading the file not always updates it
func (p *Pruner) Touch(binPath string) {
	if p.hwm > 0 {
		p.pruneLock.Lock()
		defer p.pruneLock.Unlock()
		_ = os.Chtimes(binPath, time.Now(), time.Now()) //nolint:forbidigo
	}
}

// scanTargets reads the cache directory and returns the list of binaries
// eligible for pruning together with the total cache size.
func (p *Pruner) scanTargets() ([]pruneTarget, int64, error) {
	binaries, err := os.ReadDir(p.dir) //nolint:forbidigo
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrPruningCache, err)
	}

	var (
		cacheSize    int64
		pruneTargets []pruneTarget
	)
	for _, binDir := range binaries {
		// skip any spurious file, each binary is in a directory
		if !binDir.IsDir() {
			continue
		}
		binPath := filepath.Join(p.dir, binDir.Name(), k6Binary)
		binInfo, err := os.Stat(binPath) //nolint:forbidigo
		if err != nil {
			continue
		}
		cacheSize += binInfo.Size()
		pruneTargets = append(pruneTargets, pruneTarget{
			path:      filepath.Dir(binPath), // we are going to prune the directory
			size:      binInfo.Size(),
			timestamp: binInfo.ModTime(),
		})
	}
	return pruneTargets, cacheSize, nil
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

	pruneTargets, cacheSize, err := p.scanTargets()
	if err != nil {
		return err
	}

	if cacheSize <= p.hwm {
		return nil
	}

	p.logger.Info("Pruning binary cache",
		"dir", p.dir,
		"cache_size", cacheSize,
		"limit", p.hwm,
		"entries", len(pruneTargets),
	)

	sort.Slice(pruneTargets, func(i, j int) bool {
		return pruneTargets[i].timestamp.Before(pruneTargets[j].timestamp)
	})

	errs := []error{ErrPruningCache}
	for _, target := range pruneTargets {
		if err := os.RemoveAll(target.path); err != nil { //nolint:forbidigo
			errs = append(errs, err)
			continue
		}

		p.logger.Debug("Pruned cached binary",
			"path", target.path,
			"size", target.size,
		)
		cacheSize -= target.size
		if cacheSize <= p.hwm {
			return nil
		}
	}

	return fmt.Errorf("%w cache could not be pruned", errors.Join(errs...))
}
