package fasthttp

import (
	"bufio"
	"bytes"
	"io"
	"sync"

	"github.com/valyala/bytebufferpool"
)

type headerInterface interface {
	ContentLength() int
	ReadTrailer(r *bufio.Reader) error
}

type requestStream struct {
	header          headerInterface
	prefetchedBytes *bytes.Reader
	reader          *bufio.Reader
	totalBytesRead  int
	chunkLeft       int
}

func (rs *requestStream) Read(p []byte) (int, error) {
	var (
		n   int
		err error
	)
	if rs.header.ContentLength() == -1 {
		if rs.chunkLeft == 0 {
			chunkSize, err := parseChunkSize(rs.reader)
			if err != nil {
				return 0, err
			}
			if chunkSize == 0 {
				err = rs.header.ReadTrailer(rs.reader)
				if err != nil && err != io.EOF {
					return 0, err
				}
				return 0, io.EOF
			}
			rs.chunkLeft = chunkSize
		}
		bytesToRead := len(p)
		if rs.chunkLeft < len(p) {
			bytesToRead = rs.chunkLeft
		}
		n, err = rs.reader.Read(p[:bytesToRead])
		rs.totalBytesRead += n
		rs.chunkLeft -= n
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err == nil && rs.chunkLeft == 0 {
			err = readCrLf(rs.reader)
		}
		return n, err
	}
	if rs.totalBytesRead == rs.header.ContentLength() {
		return 0, io.EOF
	}
	prefetchedSize := int(rs.prefetchedBytes.Size())
	if prefetchedSize > rs.totalBytesRead {
		left := prefetchedSize - rs.totalBytesRead
		if len(p) > left {
			p = p[:left]
		}
		n, err := rs.prefetchedBytes.Read(p)
		rs.totalBytesRead += n
		if n == rs.header.ContentLength() {
			return n, io.EOF
		}
		return n, err
	} else {
		left := rs.header.ContentLength() - rs.totalBytesRead
		if len(p) > left {
			p = p[:left]
		}
		n, err = rs.reader.Read(p)
		rs.totalBytesRead += n
		if err != nil {
			return n, err
		}
	}

	if rs.totalBytesRead == rs.header.ContentLength() {
		err = io.EOF
	}
	return n, err
}

func acquireRequestStream(b *bytebufferpool.ByteBuffer, r *bufio.Reader, h headerInterface) *requestStream {
	rs := requestStreamPool.Get().(*requestStream)
	rs.prefetchedBytes = bytes.NewReader(b.B)
	rs.reader = r
	rs.header = h
	return rs
}

func releaseRequestStream(rs *requestStream) {
	rs.prefetchedBytes = nil
	rs.totalBytesRead = 0
	rs.chunkLeft = 0
	rs.reader = nil
	rs.header = nil
	requestStreamPool.Put(rs)
}

var requestStreamPool = sync.Pool{
	New: func() interface{} {
		return &requestStream{}
	},
}
