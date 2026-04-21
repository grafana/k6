package flate

import (
	"io"

	"github.com/andybalholm/brotli/matchfinder"
)

// NewWriter returns a new matchfinder.Writer that compresses data at the given level,
// in flate encoding. Levels 1–9 are available; levels outside this range will
// be replaced with the closest level available.
func NewWriter(w io.Writer, level int) *matchfinder.Writer {
	return newWriter(w, level, NewEncoder())
}

// NewGZIPWriter returns a new matchfinder.Writer that compresses data at the given
// level, in gzip encoding. Levels 1–9 are available; levels outside this range
// will be replaced by the closest level available.
func NewGZIPWriter(w io.Writer, level int) *matchfinder.Writer {
	return newWriter(w, level, NewGZIPEncoder())
}

func newWriter(w io.Writer, level int, e matchfinder.Encoder) *matchfinder.Writer {
	var mf matchfinder.MatchFinder
	if level < 2 {
		mf = &matchfinder.ZFast{MaxDistance: 1 << 15}
	} else if level == 2 {
		mf = &matchfinder.ZDFast{MaxDistance: 1 << 15}
	} else if level == 3 {
		mf = &matchfinder.ZM{MaxDistance: 1 << 15}
	} else if level == 4 {
		mf = &matchfinder.Trio{MaxDistance: 1 << 15}
	} else if level < 8 {
		chainLen := 32
		switch level {
		case 5:
			chainLen = 8
		case 6:
			chainLen = 16
		}
		mf = &matchfinder.M4{
			MaxDistance:     1 << 15,
			ChainLength:     chainLen,
			HashLen:         5,
			DistanceBitCost: 66,
		}
	} else {
		chainLen := 32
		hashLen := 5
		if level == 8 {
			chainLen = 4
		}
		mf = &matchfinder.Pathfinder{
			MaxDistance: 1 << 15,
			ChainLength: chainLen,
			HashLen:     hashLen,
		}
	}

	return &matchfinder.Writer{
		Dest:        w,
		MatchFinder: mf,
		Encoder:     e,
		BlockSize:   1 << 16,
	}
}
