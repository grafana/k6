package k6provider

import (
	"errors"
	"time"
)

const (
	defaultBackoff = 1 * time.Second
)

var (
	// errLocked is returned when the file is already locked
	errLocked = errors.New("file locked")
	// errLockFailed is returned when there's an error accessing the lock file
	errLockFailed = errors.New("failed to lock file")
	// errUnLockFailed is returned when there's an error unlocking the file
	errUnLockFailed = errors.New("failed to lock file")
)

// lock places an advisory write lock on the directory.
// returns ErrLocked immediately if the directory is locked and the timeout expires.
// if timeout is 0, lock will wait indefinitely.
// If lock returns nil, no other process will be able to place a lock until this process exits or unlocks it.
//
// This is an portable implementation that requires an operating system specific implementation or a non-blocking
// tryLock.
// Implementing the blocking lock functionality in an operating system specific way would be more complicated and
// error prone.
func (m *dirLock) lock(timeout time.Duration) error {
	backoff := defaultBackoff
	deadLine := time.Now().Add(timeout)
	for {
		err := m.tryLock()
		if errors.Is(err, errLocked) {
			if timeout != 0 && time.Now().After(deadLine) {
				return errLocked
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return err
	}
}
