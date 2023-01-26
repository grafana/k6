package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBufferPool(t *testing.T) {
	t.Parallel()
	pool := NewBufferPool()
	// Iterate more than one to avoid GC runs
	for i := 1; i < 10; i++ {
		b := pool.Get()
		b.WriteString("test")
		require.Equal(t, "test", b.String())
		pool.Put(b)
	}
}
