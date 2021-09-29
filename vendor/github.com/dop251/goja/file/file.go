// Package file encapsulates the file abstractions used by the ast & parser.
//
package file

import (
	"fmt"
	"path"
	"sort"
	"sync"

	"github.com/go-sourcemap/sourcemap"
)

// Idx is a compact encoding of a source position within a file set.
// It can be converted into a Position for a more convenient, but much
// larger, representation.
type Idx int

// Position describes an arbitrary source position
// including the filename, line, and column location.
type Position struct {
	Filename string // The filename where the error occurred, if any
	Line     int    // The line number, starting at 1
	Column   int    // The column number, starting at 1 (The character count)

}

// A Position is valid if the line number is > 0.

func (self *Position) isValid() bool {
	return self.Line > 0
}

// String returns a string in one of several forms:
//
//	file:line:column    A valid position with filename
//	line:column         A valid position without filename
//	file                An invalid position with filename
//	-                   An invalid position without filename
//
func (self Position) String() string {
	str := self.Filename
	if self.isValid() {
		if str != "" {
			str += ":"
		}
		str += fmt.Sprintf("%d:%d", self.Line, self.Column)
	}
	if str == "" {
		str = "-"
	}
	return str
}

// FileSet

// A FileSet represents a set of source files.
type FileSet struct {
	files []*File
	last  *File
}

// AddFile adds a new file with the given filename and src.
//
// This an internal method, but exported for cross-package use.
func (self *FileSet) AddFile(filename, src string) int {
	base := self.nextBase()
	file := &File{
		name: filename,
		src:  src,
		base: base,
	}
	self.files = append(self.files, file)
	self.last = file
	return base
}

func (self *FileSet) nextBase() int {
	if self.last == nil {
		return 1
	}
	return self.last.base + len(self.last.src) + 1
}

func (self *FileSet) File(idx Idx) *File {
	for _, file := range self.files {
		if idx <= Idx(file.base+len(file.src)) {
			return file
		}
	}
	return nil
}

// Position converts an Idx in the FileSet into a Position.
func (self *FileSet) Position(idx Idx) Position {
	for _, file := range self.files {
		if idx <= Idx(file.base+len(file.src)) {
			return file.Position(int(idx) - file.base)
		}
	}
	return Position{}
}

type File struct {
	mu                sync.Mutex
	name              string
	src               string
	base              int // This will always be 1 or greater
	sourceMap         *sourcemap.Consumer
	lineOffsets       []int
	lastScannedOffset int
}

func NewFile(filename, src string, base int) *File {
	return &File{
		name: filename,
		src:  src,
		base: base,
	}
}

func (fl *File) Name() string {
	return fl.name
}

func (fl *File) Source() string {
	return fl.src
}

func (fl *File) Base() int {
	return fl.base
}

func (fl *File) SetSourceMap(m *sourcemap.Consumer) {
	fl.sourceMap = m
}

func (fl *File) Position(offset int) Position {
	var line int
	var lineOffsets []int
	fl.mu.Lock()
	if offset > fl.lastScannedOffset {
		line = fl.scanTo(offset)
		lineOffsets = fl.lineOffsets
		fl.mu.Unlock()
	} else {
		lineOffsets = fl.lineOffsets
		fl.mu.Unlock()
		line = sort.Search(len(lineOffsets), func(x int) bool { return lineOffsets[x] > offset }) - 1
	}

	var lineStart int
	if line >= 0 {
		lineStart = lineOffsets[line]
	}

	row := line + 2
	col := offset - lineStart + 1

	if fl.sourceMap != nil {
		if source, _, row, col, ok := fl.sourceMap.Source(row, col); ok {
			if !path.IsAbs(source) {
				source = path.Join(path.Dir(fl.name), source)
			}
			return Position{
				Filename: source,
				Line:     row,
				Column:   col,
			}
		}
	}

	return Position{
		Filename: fl.name,
		Line:     row,
		Column:   col,
	}
}

func findNextLineStart(s string) int {
	for pos, ch := range s {
		switch ch {
		case '\r':
			if pos < len(s)-1 && s[pos+1] == '\n' {
				return pos + 2
			}
			return pos + 1
		case '\n':
			return pos + 1
		case '\u2028', '\u2029':
			return pos + 3
		}
	}
	return -1
}

func (fl *File) scanTo(offset int) int {
	o := fl.lastScannedOffset
	for o < offset {
		p := findNextLineStart(fl.src[o:])
		if p == -1 {
			fl.lastScannedOffset = len(fl.src)
			return len(fl.lineOffsets) - 1
		}
		o = o + p
		fl.lineOffsets = append(fl.lineOffsets, o)
	}
	fl.lastScannedOffset = o

	if o == offset {
		return len(fl.lineOffsets) - 1
	}

	return len(fl.lineOffsets) - 2
}
