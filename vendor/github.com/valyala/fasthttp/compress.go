package fasthttp

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp/stackless"
)

// Supported compression levels.
const (
	CompressNoCompression      = flate.NoCompression
	CompressBestSpeed          = flate.BestSpeed
	CompressBestCompression    = flate.BestCompression
	CompressDefaultCompression = 6  // flate.DefaultCompression
	CompressHuffmanOnly        = -2 // flate.HuffmanOnly
)

func acquireGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func releaseGzipReader(zr *gzip.Reader) {
	zr.Close()
	gzipReaderPool.Put(zr)
}

var gzipReaderPool sync.Pool

func acquireFlateReader(r io.Reader) (io.ReadCloser, error) {
	v := flateReaderPool.Get()
	if v == nil {
		zr, err := zlib.NewReader(r)
		if err != nil {
			return nil, err
		}
		return zr, nil
	}
	zr := v.(io.ReadCloser)
	if err := resetFlateReader(zr, r); err != nil {
		return nil, err
	}
	return zr, nil
}

func releaseFlateReader(zr io.ReadCloser) {
	zr.Close()
	flateReaderPool.Put(zr)
}

func resetFlateReader(zr io.ReadCloser, r io.Reader) error {
	zrr, ok := zr.(zlib.Resetter)
	if !ok {
		// sanity check. should only be called with a zlib.Reader
		panic("BUG: zlib.Reader doesn't implement zlib.Resetter???")
	}
	return zrr.Reset(r, nil)
}

var flateReaderPool sync.Pool

func acquireStacklessGzipWriter(w io.Writer, level int) stackless.Writer {
	nLevel := normalizeCompressLevel(level)
	p := stacklessGzipWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		return stackless.NewWriter(w, func(w io.Writer) stackless.Writer {
			return acquireRealGzipWriter(w, level)
		})
	}
	sw := v.(stackless.Writer)
	sw.Reset(w)
	return sw
}

func releaseStacklessGzipWriter(sw stackless.Writer, level int) {
	sw.Close()
	nLevel := normalizeCompressLevel(level)
	p := stacklessGzipWriterPoolMap[nLevel]
	p.Put(sw)
}

func acquireRealGzipWriter(w io.Writer, level int) *gzip.Writer {
	nLevel := normalizeCompressLevel(level)
	p := realGzipWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		zw, err := gzip.NewWriterLevel(w, level)
		if err != nil {
			// gzip.NewWriterLevel only errors for invalid
			// compression levels. Clamp it to be min or max.
			if level < gzip.HuffmanOnly {
				level = gzip.HuffmanOnly
			} else {
				level = gzip.BestCompression
			}
			zw, _ = gzip.NewWriterLevel(w, level)
		}
		return zw
	}
	zw := v.(*gzip.Writer)
	zw.Reset(w)
	return zw
}

func releaseRealGzipWriter(zw *gzip.Writer, level int) {
	zw.Close()
	nLevel := normalizeCompressLevel(level)
	p := realGzipWriterPoolMap[nLevel]
	p.Put(zw)
}

var (
	stacklessGzipWriterPoolMap = newCompressWriterPoolMap()
	realGzipWriterPoolMap      = newCompressWriterPoolMap()
)

// AppendGzipBytesLevel appends gzipped src to dst using the given
// compression level and returns the resulting dst.
//
// Supported compression levels are:
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func AppendGzipBytesLevel(dst, src []byte, level int) []byte {
	w := &byteSliceWriter{dst}
	WriteGzipLevel(w, src, level) //nolint:errcheck
	return w.b
}

// WriteGzipLevel writes gzipped p to w using the given compression level
// and returns the number of compressed bytes written to w.
//
// Supported compression levels are:
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func WriteGzipLevel(w io.Writer, p []byte, level int) (int, error) {
	switch w.(type) {
	case *byteSliceWriter,
		*bytes.Buffer,
		*bytebufferpool.ByteBuffer:
		// These writers don't block, so we can just use stacklessWriteGzip
		ctx := &compressCtx{
			w:     w,
			p:     p,
			level: level,
		}
		stacklessWriteGzip(ctx)
		return len(p), nil
	default:
		zw := acquireStacklessGzipWriter(w, level)
		n, err := zw.Write(p)
		releaseStacklessGzipWriter(zw, level)
		return n, err
	}
}

var stacklessWriteGzip = stackless.NewFunc(nonblockingWriteGzip)

func nonblockingWriteGzip(ctxv interface{}) {
	ctx := ctxv.(*compressCtx)
	zw := acquireRealGzipWriter(ctx.w, ctx.level)

	zw.Write(ctx.p) //nolint:errcheck // no way to handle this error anyway

	releaseRealGzipWriter(zw, ctx.level)
}

// WriteGzip writes gzipped p to w and returns the number of compressed
// bytes written to w.
func WriteGzip(w io.Writer, p []byte) (int, error) {
	return WriteGzipLevel(w, p, CompressDefaultCompression)
}

// AppendGzipBytes appends gzipped src to dst and returns the resulting dst.
func AppendGzipBytes(dst, src []byte) []byte {
	return AppendGzipBytesLevel(dst, src, CompressDefaultCompression)
}

// WriteGunzip writes ungzipped p to w and returns the number of uncompressed
// bytes written to w.
func WriteGunzip(w io.Writer, p []byte) (int, error) {
	r := &byteSliceReader{p}
	zr, err := acquireGzipReader(r)
	if err != nil {
		return 0, err
	}
	n, err := copyZeroAlloc(w, zr)
	releaseGzipReader(zr)
	nn := int(n)
	if int64(nn) != n {
		return 0, fmt.Errorf("too much data gunzipped: %d", n)
	}
	return nn, err
}

// AppendGunzipBytes appends gunzipped src to dst and returns the resulting dst.
func AppendGunzipBytes(dst, src []byte) ([]byte, error) {
	w := &byteSliceWriter{dst}
	_, err := WriteGunzip(w, src)
	return w.b, err
}

// AppendDeflateBytesLevel appends deflated src to dst using the given
// compression level and returns the resulting dst.
//
// Supported compression levels are:
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func AppendDeflateBytesLevel(dst, src []byte, level int) []byte {
	w := &byteSliceWriter{dst}
	WriteDeflateLevel(w, src, level) //nolint:errcheck
	return w.b
}

// WriteDeflateLevel writes deflated p to w using the given compression level
// and returns the number of compressed bytes written to w.
//
// Supported compression levels are:
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func WriteDeflateLevel(w io.Writer, p []byte, level int) (int, error) {
	switch w.(type) {
	case *byteSliceWriter,
		*bytes.Buffer,
		*bytebufferpool.ByteBuffer:
		// These writers don't block, so we can just use stacklessWriteDeflate
		ctx := &compressCtx{
			w:     w,
			p:     p,
			level: level,
		}
		stacklessWriteDeflate(ctx)
		return len(p), nil
	default:
		zw := acquireStacklessDeflateWriter(w, level)
		n, err := zw.Write(p)
		releaseStacklessDeflateWriter(zw, level)
		return n, err
	}
}

var stacklessWriteDeflate = stackless.NewFunc(nonblockingWriteDeflate)

func nonblockingWriteDeflate(ctxv interface{}) {
	ctx := ctxv.(*compressCtx)
	zw := acquireRealDeflateWriter(ctx.w, ctx.level)

	zw.Write(ctx.p) //nolint:errcheck // no way to handle this error anyway

	releaseRealDeflateWriter(zw, ctx.level)
}

type compressCtx struct {
	w     io.Writer
	p     []byte
	level int
}

// WriteDeflate writes deflated p to w and returns the number of compressed
// bytes written to w.
func WriteDeflate(w io.Writer, p []byte) (int, error) {
	return WriteDeflateLevel(w, p, CompressDefaultCompression)
}

// AppendDeflateBytes appends deflated src to dst and returns the resulting dst.
func AppendDeflateBytes(dst, src []byte) []byte {
	return AppendDeflateBytesLevel(dst, src, CompressDefaultCompression)
}

// WriteInflate writes inflated p to w and returns the number of uncompressed
// bytes written to w.
func WriteInflate(w io.Writer, p []byte) (int, error) {
	r := &byteSliceReader{p}
	zr, err := acquireFlateReader(r)
	if err != nil {
		return 0, err
	}
	n, err := copyZeroAlloc(w, zr)
	releaseFlateReader(zr)
	nn := int(n)
	if int64(nn) != n {
		return 0, fmt.Errorf("too much data inflated: %d", n)
	}
	return nn, err
}

// AppendInflateBytes appends inflated src to dst and returns the resulting dst.
func AppendInflateBytes(dst, src []byte) ([]byte, error) {
	w := &byteSliceWriter{dst}
	_, err := WriteInflate(w, src)
	return w.b, err
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

type byteSliceReader struct {
	b []byte
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func (r *byteSliceReader) ReadByte() (byte, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := r.b[0]
	r.b = r.b[1:]
	return n, nil
}

func acquireStacklessDeflateWriter(w io.Writer, level int) stackless.Writer {
	nLevel := normalizeCompressLevel(level)
	p := stacklessDeflateWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		return stackless.NewWriter(w, func(w io.Writer) stackless.Writer {
			return acquireRealDeflateWriter(w, level)
		})
	}
	sw := v.(stackless.Writer)
	sw.Reset(w)
	return sw
}

func releaseStacklessDeflateWriter(sw stackless.Writer, level int) {
	sw.Close()
	nLevel := normalizeCompressLevel(level)
	p := stacklessDeflateWriterPoolMap[nLevel]
	p.Put(sw)
}

func acquireRealDeflateWriter(w io.Writer, level int) *zlib.Writer {
	nLevel := normalizeCompressLevel(level)
	p := realDeflateWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		zw, err := zlib.NewWriterLevel(w, level)
		if err != nil {
			// zlib.NewWriterLevel only errors for invalid
			// compression levels. Clamp it to be min or max.
			if level < zlib.HuffmanOnly {
				level = zlib.HuffmanOnly
			} else {
				level = zlib.BestCompression
			}
			zw, _ = zlib.NewWriterLevel(w, level)
		}
		return zw
	}
	zw := v.(*zlib.Writer)
	zw.Reset(w)
	return zw
}

func releaseRealDeflateWriter(zw *zlib.Writer, level int) {
	zw.Close()
	nLevel := normalizeCompressLevel(level)
	p := realDeflateWriterPoolMap[nLevel]
	p.Put(zw)
}

var (
	stacklessDeflateWriterPoolMap = newCompressWriterPoolMap()
	realDeflateWriterPoolMap      = newCompressWriterPoolMap()
)

func newCompressWriterPoolMap() []*sync.Pool {
	// Initialize pools for all the compression levels defined
	// in https://pkg.go.dev/compress/flate#pkg-constants .
	// Compression levels are normalized with normalizeCompressLevel,
	// so the fit [0..11].
	var m []*sync.Pool
	for i := 0; i < 12; i++ {
		m = append(m, &sync.Pool{})
	}
	return m
}

func isFileCompressible(f *os.File, minCompressRatio float64) bool {
	// Try compressing the first 4kb of the file
	// and see if it can be compressed by more than
	// the given minCompressRatio.
	b := bytebufferpool.Get()
	zw := acquireStacklessGzipWriter(b, CompressDefaultCompression)
	lr := &io.LimitedReader{
		R: f,
		N: 4096,
	}
	_, err := copyZeroAlloc(zw, lr)
	releaseStacklessGzipWriter(zw, CompressDefaultCompression)
	f.Seek(0, 0) //nolint:errcheck
	if err != nil {
		return false
	}

	n := 4096 - lr.N
	zn := len(b.B)
	bytebufferpool.Put(b)
	return float64(zn) < float64(n)*minCompressRatio
}

// normalizes compression level into [0..11], so it could be used as an index
// in *PoolMap.
func normalizeCompressLevel(level int) int {
	// -2 is the lowest compression level - CompressHuffmanOnly
	// 9 is the highest compression level - CompressBestCompression
	if level < -2 || level > 9 {
		level = CompressDefaultCompression
	}
	return level + 2
}
