package fasthttp

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp/stackless"
)

// Supported compression levels.
const (
	CompressBrotliNoCompression   = 0
	CompressBrotliBestSpeed       = brotli.BestSpeed
	CompressBrotliBestCompression = brotli.BestCompression

	// Choose a default brotli compression level comparable to
	// CompressDefaultCompression (gzip 6)
	// See: https://github.com/valyala/fasthttp/issues/798#issuecomment-626293806
	CompressBrotliDefaultCompression = 4
)

func acquireBrotliReader(r io.Reader) (*brotli.Reader, error) {
	v := brotliReaderPool.Get()
	if v == nil {
		return brotli.NewReader(r), nil
	}
	zr := v.(*brotli.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func releaseBrotliReader(zr *brotli.Reader) {
	brotliReaderPool.Put(zr)
}

var brotliReaderPool sync.Pool

func acquireStacklessBrotliWriter(w io.Writer, level int) stackless.Writer {
	nLevel := normalizeBrotliCompressLevel(level)
	p := stacklessBrotliWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		return stackless.NewWriter(w, func(w io.Writer) stackless.Writer {
			return acquireRealBrotliWriter(w, level)
		})
	}
	sw := v.(stackless.Writer)
	sw.Reset(w)
	return sw
}

func releaseStacklessBrotliWriter(sw stackless.Writer, level int) {
	sw.Close()
	nLevel := normalizeBrotliCompressLevel(level)
	p := stacklessBrotliWriterPoolMap[nLevel]
	p.Put(sw)
}

func acquireRealBrotliWriter(w io.Writer, level int) *brotli.Writer {
	nLevel := normalizeBrotliCompressLevel(level)
	p := realBrotliWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		zw := brotli.NewWriterLevel(w, level)
		return zw
	}
	zw := v.(*brotli.Writer)
	zw.Reset(w)
	return zw
}

func releaseRealBrotliWriter(zw *brotli.Writer, level int) {
	zw.Close()
	nLevel := normalizeBrotliCompressLevel(level)
	p := realBrotliWriterPoolMap[nLevel]
	p.Put(zw)
}

var (
	stacklessBrotliWriterPoolMap = newCompressWriterPoolMap()
	realBrotliWriterPoolMap      = newCompressWriterPoolMap()
)

// AppendBrotliBytesLevel appends brotlied src to dst using the given
// compression level and returns the resulting dst.
//
// Supported compression levels are:
//
//   - CompressBrotliNoCompression
//   - CompressBrotliBestSpeed
//   - CompressBrotliBestCompression
//   - CompressBrotliDefaultCompression
func AppendBrotliBytesLevel(dst, src []byte, level int) []byte {
	w := &byteSliceWriter{dst}
	WriteBrotliLevel(w, src, level) //nolint:errcheck
	return w.b
}

// WriteBrotliLevel writes brotlied p to w using the given compression level
// and returns the number of compressed bytes written to w.
//
// Supported compression levels are:
//
//   - CompressBrotliNoCompression
//   - CompressBrotliBestSpeed
//   - CompressBrotliBestCompression
//   - CompressBrotliDefaultCompression
func WriteBrotliLevel(w io.Writer, p []byte, level int) (int, error) {
	switch w.(type) {
	case *byteSliceWriter,
		*bytes.Buffer,
		*bytebufferpool.ByteBuffer:
		// These writers don't block, so we can just use stacklessWriteBrotli
		ctx := &compressCtx{
			w:     w,
			p:     p,
			level: level,
		}
		stacklessWriteBrotli(ctx)
		return len(p), nil
	default:
		zw := acquireStacklessBrotliWriter(w, level)
		n, err := zw.Write(p)
		releaseStacklessBrotliWriter(zw, level)
		return n, err
	}
}

var stacklessWriteBrotli = stackless.NewFunc(nonblockingWriteBrotli)

func nonblockingWriteBrotli(ctxv interface{}) {
	ctx := ctxv.(*compressCtx)
	zw := acquireRealBrotliWriter(ctx.w, ctx.level)

	zw.Write(ctx.p) //nolint:errcheck // no way to handle this error anyway

	releaseRealBrotliWriter(zw, ctx.level)
}

// WriteBrotli writes brotlied p to w and returns the number of compressed
// bytes written to w.
func WriteBrotli(w io.Writer, p []byte) (int, error) {
	return WriteBrotliLevel(w, p, CompressBrotliDefaultCompression)
}

// AppendBrotliBytes appends brotlied src to dst and returns the resulting dst.
func AppendBrotliBytes(dst, src []byte) []byte {
	return AppendBrotliBytesLevel(dst, src, CompressBrotliDefaultCompression)
}

// WriteUnbrotli writes unbrotlied p to w and returns the number of uncompressed
// bytes written to w.
func WriteUnbrotli(w io.Writer, p []byte) (int, error) {
	r := &byteSliceReader{p}
	zr, err := acquireBrotliReader(r)
	if err != nil {
		return 0, err
	}
	n, err := copyZeroAlloc(w, zr)
	releaseBrotliReader(zr)
	nn := int(n)
	if int64(nn) != n {
		return 0, fmt.Errorf("too much data unbrotlied: %d", n)
	}
	return nn, err
}

// AppendUnbrotliBytes appends unbrotlied src to dst and returns the resulting dst.
func AppendUnbrotliBytes(dst, src []byte) ([]byte, error) {
	w := &byteSliceWriter{dst}
	_, err := WriteUnbrotli(w, src)
	return w.b, err
}

// normalizes compression level into [0..11], so it could be used as an index
// in *PoolMap.
func normalizeBrotliCompressLevel(level int) int {
	// -2 is the lowest compression level - CompressHuffmanOnly
	// 9 is the highest compression level - CompressBestCompression
	if level < 0 || level > 11 {
		level = CompressBrotliDefaultCompression
	}
	return level
}
