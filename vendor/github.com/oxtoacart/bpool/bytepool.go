package bpool

// BytePool implements a leaky pool of []byte in the form of a bounded
// channel.
type BytePool struct {
	c chan []byte
	w int
	h int
}

// NewBytePool creates a new BytePool bounded to the given maxSize, with new
// byte arrays sized based on width.
func NewBytePool(maxSize int, width int) (bp *BytePool) {
	return &BytePool{
		c: make(chan []byte, maxSize),
		w: width,
	}
}

// Get gets a []byte from the BytePool, or creates a new one if none are
// available in the pool.
func (bp *BytePool) Get() (b []byte) {
	select {
	case b = <-bp.c:
	// reuse existing buffer
	default:
		// create new buffer
		b = make([]byte, bp.w)
	}
	return
}

// Put returns the given Buffer to the BytePool.
func (bp *BytePool) Put(b []byte) {
	if cap(b) < bp.w {
		// someone tried to put back a too small buffer, discard it
		return
	}

	select {
	case bp.c <- b[:bp.w]:
		// buffer went back into pool
	default:
		// buffer didn't go back into pool, just discard
	}
}

// NumPooled returns the number of items currently pooled.
func (bp *BytePool) NumPooled() int {
	return len(bp.c)
}

// Width returns the width of the byte arrays in this pool.
func (bp *BytePool) Width() (n int) {
	return bp.w
}
