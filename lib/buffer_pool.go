package lib

import (
	"bytes"
	"sync"
)

// BufferPool implements a bytes.Buffer pool using sync.Pool
type BufferPool struct {
	pool *sync.Pool
}

// NewBufferPool create a new instance of BufferPool using a sync.Pool implementation
// returning a bytes.NewBuffer for each pooled new element
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer([]byte{})
			},
		},
	}
}

// Get return a bytes.Buffer from the pool
//
//nolint:forcetypeassert
func (bp BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put return the given bytes.Buffer to the pool calling Buffer.Reset() before
func (bp BufferPool) Put(b *bytes.Buffer) {
	// Important to clean the current data from de buffer for the next use,
	// otherwise a dirty buffer with the current data will be returned
	b.Reset()
	bp.pool.Put(b)
}
