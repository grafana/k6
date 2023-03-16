package platform

// bufWriter implements io.Writer.
//
// This is implemented because bytes.Buffer cannot write from the beginning of the underlying buffer
// without changing the memory location. In this case, the underlying buffer is memory-mapped region,
// and we have to write into that region via io.Copy since sometimes the original native code exists
// as a file for external-cached cases.
type bufWriter struct {
	underlying []byte
	pos        int
}

// Write implements io.Writer Write.
func (b *bufWriter) Write(p []byte) (n int, err error) {
	copy(b.underlying[b.pos:], p)
	n = len(p)
	b.pos += n
	return
}
