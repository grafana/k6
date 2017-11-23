package goja

import (
	"fmt"
	"sort"
	"strings"
)

type Position struct {
	Line, Col int
}

type SrcFile struct {
	name string
	src  string

	lineOffsets       []int
	lastScannedOffset int
}

func NewSrcFile(name, src string) *SrcFile {
	return &SrcFile{
		name: name,
		src:  src,
	}
}

func (f *SrcFile) Position(offset int) Position {
	var line int
	if offset > f.lastScannedOffset {
		f.scanTo(offset)
		line = len(f.lineOffsets) - 1
	} else {
		if len(f.lineOffsets) > 0 {
			line = sort.SearchInts(f.lineOffsets, offset)
		} else {
			line = -1
		}
	}

	if line >= 0 {
		if f.lineOffsets[line] > offset {
			line--
		}
	}

	var lineStart int
	if line >= 0 {
		lineStart = f.lineOffsets[line]
	}
	return Position{
		Line: line + 2,
		Col:  offset - lineStart + 1,
	}
}

func (f *SrcFile) scanTo(offset int) {
	o := f.lastScannedOffset
	for o < offset {
		p := strings.Index(f.src[o:], "\n")
		if p == -1 {
			o = len(f.src)
			break
		}
		o = o + p + 1
		f.lineOffsets = append(f.lineOffsets, o)
	}
	f.lastScannedOffset = o
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}
