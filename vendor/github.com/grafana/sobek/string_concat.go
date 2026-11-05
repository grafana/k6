package sobek

import (
	"hash/maphash"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/grafana/sobek/unistring"
)

const concatThreshold = 32

type concatString struct {
	// When flattened left contains the flat string, right is nil
	left   String
	right  String
	length int
	// Strings should be treated as immutable therefore they can be shared by multiple Runtimes
	mu sync.Mutex
}

func (s *concatString) flatten() (flattened String) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.right != nil {
		var sb StringBuilder
		s.writeTo(&sb)
		flattened = sb.String()
		s.left, s.right = flattened, nil
	} else {
		flattened = s.left
	}

	return flattened
}

func (s *concatString) load() (left, right String) {
	s.mu.Lock()
	left, right = s.left, s.right
	s.mu.Unlock()
	return
}

// must be called with s.mu locked
func (s *concatString) writeTo(sb *StringBuilder) {
	var grew, unicodeDetected bool
	stack := make([]String, 0, 64)
	stack = append(stack, s.right, s.left)
	if _, ok := s.right.(unicodeString); ok {
		unicodeDetected = true
	}
	for len(stack) > 0 {
		n := len(stack) - 1
		str := stack[n]
		stack = stack[:n]

		if cs, ok := str.(*concatString); ok {
			left, right := cs.load()
			if right == nil {
				str = left
			} else {
				if !grew && !unicodeDetected {
					if _, ok := right.(unicodeString); ok {
						unicodeDetected = true
					}
				}
				stack = append(stack, right, left)
				continue
			}
		}
		if !grew {
			if !unicodeDetected {
				if _, ok := str.(unicodeString); ok {
					unicodeDetected = true
				}
			}
			if unicodeDetected {
				sb.LikelyUnicode(s.length)
			} else {
				sb.Grow(s.length)
			}
			grew = true
		}
		sb.WriteString(str)
	}
}

func (s *concatString) ToInteger() int64 {
	return s.flatten().ToInteger()
}

func (s *concatString) toString() String {
	if left, right := s.load(); right == nil {
		return left
	}
	return s
}

func (s *concatString) string() unistring.String {
	return s.flatten().string()
}

func (s *concatString) ToString() Value {
	if left, right := s.load(); right == nil {
		return left
	}
	return s
}

func (s *concatString) String() string {
	return s.flatten().String()
}

func (s *concatString) ToFloat() float64 {
	return s.flatten().ToFloat()
}

func (s *concatString) ToNumber() Value {
	return s.flatten().ToNumber()
}

func (s *concatString) ToBoolean() bool {
	return s.length > 0
}

func (s *concatString) ToObject(r *Runtime) *Object {
	return r._newString(s.toString(), r.getStringPrototype())
}

func (s *concatString) SameAs(other Value) bool {
	return s.StrictEquals(other)
}

func (s *concatString) Equals(other Value) bool {
	if s == other {
		return true
	}
	return s.flatten().Equals(other)
}

func (s *concatString) StrictEquals(other Value) bool {
	if s == other {
		return true
	}
	return s.flatten().StrictEquals(other)
}

func (s *concatString) baseObject(r *Runtime) *Object {
	if left, right := s.load(); right == nil {
		return left.baseObject(r)
	}
	ss := r.getStringSingleton()
	ss.value = s
	ss.setLength()
	return ss.val
}

func (s *concatString) Export() interface{} {
	return s.String()
}

func (s *concatString) ExportType() reflect.Type {
	return reflectTypeString
}

func (s *concatString) hash(h *maphash.Hash) uint64 {
	return s.flatten().hash(h)
}

func (s *concatString) CharAt(idx int) uint16 {
	return s.flatten().CharAt(idx)
}

func (s *concatString) Length() int {
	return s.length
}

func (s *concatString) Concat(other String) String {
	totalLen := s.length + other.Length()
	if totalLen <= concatThreshold {
		return s.flatten().Concat(other)
	}
	return &concatString{
		left:   s,
		right:  other,
		length: totalLen,
	}
}

func (s *concatString) Substring(start, end int) String {
	return s.flatten().Substring(start, end)
}

func (s *concatString) CompareTo(other String) int {
	return s.flatten().CompareTo(other)
}

func (s *concatString) Reader() io.RuneReader {
	return s.flatten().Reader()
}

func (s *concatString) utf16Reader() utf16Reader {
	return s.flatten().utf16Reader()
}

func (s *concatString) utf16RuneReader() io.RuneReader {
	return s.flatten().utf16RuneReader()
}

func (s *concatString) utf16Runes() []rune {
	return s.flatten().utf16Runes()
}

func (s *concatString) index(substr String, start int) int {
	return s.flatten().index(substr, start)
}

func (s *concatString) lastIndex(substr String, pos int) int {
	return s.flatten().lastIndex(substr, pos)
}

func (s *concatString) toLower() String {
	return s.flatten().toLower()
}

func (s *concatString) toUpper() String {
	return s.flatten().toUpper()
}

func (s *concatString) toTrimmedUTF8() string {
	return strings.Trim(s.String(), WhitespaceChars)
}
