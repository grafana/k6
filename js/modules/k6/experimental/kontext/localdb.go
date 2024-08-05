package kontext

import (
	"fmt"
	"sync"
	"sync/atomic"

	bolt "go.etcd.io/bbolt"
)

const (
	// DefaultLocalKontextPath is the default path to the KV store
	DefaultLocalKontextPath = ".k6.kontext"

	// DefaultLocalKontextBucket is the default bucket name for the KV store
	DefaultLocalKontextBucket = "k6"
)

// db is a wrapper around bolt.DB that keeps track of the number of references
// to the database and closes the database when the last reference is closed.
type db struct {
	path     string
	handle   *bolt.DB
	opened   atomic.Bool
	refCount atomic.Int64
	lock     sync.Mutex
}

// newDB returns a new db instance.
func newDB() *db {
	return &db{
		path:     DefaultLocalKontextPath,
		handle:   new(bolt.DB),
		opened:   atomic.Bool{},
		refCount: atomic.Int64{},
		lock:     sync.Mutex{},
	}
}

// open opens the database if it is not already open.
//
// It is safe to call this method multiple times.
// The database will only be opened once.
func (db *db) open() error {
	if db.opened.Load() {
		db.refCount.Add(1)
		return nil
	}

	db.lock.Lock()
	defer db.lock.Unlock()

	if db.opened.Load() {
		return nil
	}

	handler, err := bolt.Open(db.path, 0o600, nil)
	if err != nil {
		return err
	}

	err = handler.Update(func(tx *bolt.Tx) error {
		_, bucketErr := tx.CreateBucketIfNotExists([]byte(DefaultLocalKontextBucket))
		if bucketErr != nil {
			return fmt.Errorf("failed to create internal bucket: %w", bucketErr)
		}

		return nil
	})
	if err != nil {
		return err
	}

	db.handle = handler
	db.opened.Store(true)
	db.refCount.Add(1)

	return nil
}

// close closes the database if there are no more references to it.
func (db *db) close() error {
	if db.refCount.Add(-1) == 0 {
		if err := db.handle.Close(); err != nil {
			return err
		}

		db.handle = nil
		db.opened.Store(false)
	}

	return nil
}
