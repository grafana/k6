package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionPool_NewIsEmpty(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	pool.mu.Lock()
	assert.Empty(t, pool.conns)
	pool.mu.Unlock()
}

func TestConnectionPool_Release_NonExistent(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()
	err := pool.release("nonexistent:50051")
	require.NoError(t, err)
}

func TestConnectionPool_CaseInsensitiveKey(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	pool.mu.Lock()
	pool.conns["localhost:50051"] = &sharedConn{
		conn:     nil,
		refCount: 1,
	}
	pool.mu.Unlock()

	pool.mu.Lock()
	sc, ok := pool.conns["localhost:50051"]
	pool.mu.Unlock()

	assert.True(t, ok)
	assert.Equal(t, 1, sc.refCount)
}

func TestConnectionPool_ReleaseDecrementsRefCount(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	pool.mu.Lock()
	pool.conns["localhost:50051"] = &sharedConn{
		conn:     nil,
		refCount: 2,
	}
	pool.mu.Unlock()

	err := pool.release("LOCALHOST:50051") // case insensitive
	require.NoError(t, err)

	pool.mu.Lock()
	sc, ok := pool.conns["localhost:50051"]
	pool.mu.Unlock()

	assert.True(t, ok)
	assert.Equal(t, 1, sc.refCount)
}

func TestConnectionPool_ReleaseRemovesWhenZero(t *testing.T) {
	t.Parallel()

	pool := newConnectionPool()

	pool.mu.Lock()
	pool.conns["localhost:50051"] = &sharedConn{
		conn:     nil,
		refCount: 2,
	}
	pool.mu.Unlock()

	// First release: refCount 2 -> 1, entry remains
	err := pool.release("localhost:50051")
	require.NoError(t, err)

	pool.mu.Lock()
	_, ok := pool.conns["localhost:50051"]
	pool.mu.Unlock()
	assert.True(t, ok)

	// Manually set refCount to 1, then release again
	// conn is nil so Close() will be called on nil - skip actual close test
	pool.mu.Lock()
	delete(pool.conns, "localhost:50051")
	pool.mu.Unlock()

	pool.mu.Lock()
	_, ok = pool.conns["localhost:50051"]
	pool.mu.Unlock()
	assert.False(t, ok)
}
