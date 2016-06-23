package lib

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"testing"
)

type PoolTestVU struct {
	ID int64
}

func (u *PoolTestVU) Reconfigure(id int64) error        { u.ID = id; return nil }
func (u *PoolTestVU) RunOnce(ctx context.Context) error { return nil }

func TestGetEmptyPool(t *testing.T) {
	pool := VUPool{New: func() (VU, error) { return &PoolTestVU{}, nil }}
	assert.Equal(t, 0, pool.Count())

	vu, _ := pool.Get()
	assert.IsType(t, &PoolTestVU{}, vu)
	assert.Equal(t, 0, pool.Count())
}

func TestPutThenGet(t *testing.T) {
	pool := VUPool{New: func() (VU, error) { return &PoolTestVU{}, nil }}
	assert.Equal(t, 0, pool.Count())

	pool.Put(&PoolTestVU{ID: 1})
	assert.Equal(t, 1, pool.Count())

	vu, _ := pool.Get()
	assert.IsType(t, &PoolTestVU{}, vu)
	assert.Equal(t, int64(1), vu.(*PoolTestVU).ID)
	assert.Equal(t, 0, pool.Count())
}
