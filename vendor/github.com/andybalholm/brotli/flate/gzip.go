package flate

import (
	"hash/crc32"
	"time"

	"github.com/andybalholm/brotli/matchfinder"
)

func NewGZIPEncoder() matchfinder.Encoder {
	return &gzipEncoder{
		f: NewEncoder(),
	}
}

type gzipEncoder struct {
	f           matchfinder.Encoder
	length      uint32
	crc         uint32
	wroteHeader bool
}

func (g *gzipEncoder) Reset() {
	g.f.Reset()
	g.length = 0
	g.crc = 0
	g.wroteHeader = false
}

func appendUint32(dst []byte, n uint32) []byte {
	return append(dst,
		byte(n),
		byte(n>>8),
		byte(n>>16),
		byte(n>>24),
	)
}

func (g *gzipEncoder) Encode(dst []byte, src []byte, matches []matchfinder.Match, lastBlock bool) []byte {
	if !g.wroteHeader {
		dst = append(dst,
			0x1f, 0x8b, // magic number
			8, // CM = flate
			0, // FLG
		)
		dst = appendUint32(dst, uint32(time.Now().Unix()))
		dst = append(dst,
			0,   // XFL
			255, // OS (unspecified)
		)
		g.wroteHeader = true
	}

	dst = g.f.Encode(dst, src, matches, lastBlock)

	g.length += uint32(len(src))
	g.crc = crc32.Update(g.crc, crc32.IEEETable, src)

	if lastBlock {
		dst = appendUint32(dst, g.crc)
		dst = appendUint32(dst, g.length)
	}

	return dst
}
