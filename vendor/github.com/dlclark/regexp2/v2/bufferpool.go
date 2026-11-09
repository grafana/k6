package regexp2

import (
	"bytes"
	"slices"
	"sync"
)

type pooledSliceBuffers[T any] struct {
	sizes []int
	pools []sync.Pool
}

func newPooledSliceBuffers[T any](sizes ...int) *pooledSliceBuffers[T] {
	sizes = slices.Clone(sizes)
	slices.Sort(sizes)
	return &pooledSliceBuffers[T]{
		sizes: sizes,
		pools: make([]sync.Pool, len(sizes)),
	}
}

func (p *pooledSliceBuffers[T]) poolIndex(neededSize, maxSize int) int {
	if maxSize == 0 {
		return -1
	}
	for i, classSize := range p.sizes {
		if neededSize <= classSize {
			if maxSize > 0 && classSize > maxSize {
				return -1
			}
			return i
		}
	}
	return -1
}

func (p *pooledSliceBuffers[T]) get(neededSize, maxSize int) ([]T, *[]T) {
	idx := p.poolIndex(neededSize, maxSize)
	if idx < 0 {
		return make([]T, neededSize), nil
	}
	if v := p.pools[idx].Get(); v != nil {
		bufp := v.(*[]T)
		if cap(*bufp) >= neededSize {
			return (*bufp)[:neededSize], bufp
		}
	}
	buf := make([]T, neededSize, p.sizes[idx])
	return buf, &buf
}

func (p *pooledSliceBuffers[T]) put(bufp *[]T) {
	idx := p.poolIndex(cap(*bufp), -1)
	if idx < 0 || cap(*bufp) != p.sizes[idx] {
		return
	}
	*bufp = (*bufp)[:0]
	p.pools[idx].Put(bufp)
}

// getExact uses size classes to share slice wrappers, but allocates exactly the
// requested capacity. A returned match can then take the buffer without retaining
// a much larger size-class allocation.
func (p *pooledSliceBuffers[T]) getExact(neededSize, maxSize int) ([]T, *[]T) {
	idx := p.poolIndex(neededSize, maxSize)
	if idx < 0 {
		return make([]T, neededSize), nil
	}
	var bufp *[]T
	if v := p.pools[idx].Get(); v != nil {
		bufp = v.(*[]T)
	} else {
		bufp = new([]T)
	}
	if cap(*bufp) != neededSize {
		// Store new buffers in the wrapper only when they are returned. A
		// successful search transfers the buffer and recycles an empty wrapper.
		return make([]T, neededSize), bufp
	}
	return (*bufp)[:neededSize], bufp
}

func (p *pooledSliceBuffers[T]) putExact(bufp *[]T) {
	idx := p.poolIndex(cap(*bufp), -1)
	if idx < 0 {
		return
	}
	*bufp = (*bufp)[:0]
	p.pools[idx].Put(bufp)
}

// detach gives the caller ownership of the buffer and recycles its wrapper.
// Only getExact may borrow these wrappers, since a detached one has no buffer.
func (p *pooledSliceBuffers[T]) detach(bufp *[]T, bufferSize int) {
	idx := p.poolIndex(bufferSize, -1)
	*bufp = nil
	if idx >= 0 {
		p.pools[idx].Put(bufp)
	}
}

// our specific pooled buffers
var (
	pooledRuneBuffers = newPooledSliceBuffers[rune](1<<10, 4<<10, 16<<10, 64<<10, 256<<10)
	// Match-returning string searches can transfer their exact-size buffers to callers.
	pooledMatchRuneBuffers = newPooledSliceBuffers[rune](1<<10, 4<<10, 16<<10, 64<<10, 256<<10)
	pooledByteBuffers      = newPooledSliceBuffers[byte](4<<10, 16<<10, 64<<10, 256<<10, 1<<20)
)

func getPooledReplaceBuffer(neededBytes, maxSize int) (*bytes.Buffer, *[]byte) {
	buf, pooled := pooledByteBuffers.get(neededBytes, maxSize)
	return bytes.NewBuffer(buf[:0]), pooled
}

func putPooledReplaceBuffer(buf *bytes.Buffer, pooled *[]byte) {
	*pooled = buf.Bytes()
	pooledByteBuffers.put(pooled)
}
