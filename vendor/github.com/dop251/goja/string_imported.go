package goja

import (
	"hash/maphash"
	"io"
	"math"
	"reflect"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/unistring"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Represents a string imported from Go. The idea is to delay the scanning for unicode characters and converting
// to unicodeString until necessary. This way strings that are merely passed through never get scanned which
// saves CPU and memory.
// Currently, importedString is created in 2 cases: Runtime.ToValue() for strings longer than 16 bytes and as a result
// of JSON.stringify() if it may contain unicode characters. More cases could be added in the future.
type importedString struct {
	s string
	u unicodeString

	scanned bool
}

func (i *importedString) scan() {
	i.u = unistring.Scan(i.s)
	i.scanned = true
}

func (i *importedString) ensureScanned() {
	if !i.scanned {
		i.scan()
	}
}

func (i *importedString) ToInteger() int64 {
	i.ensureScanned()
	if i.u != nil {
		return 0
	}
	return asciiString(i.s).ToInteger()
}

func (i *importedString) toString() valueString {
	return i
}

func (i *importedString) string() unistring.String {
	i.ensureScanned()
	if i.u != nil {
		return unistring.FromUtf16(i.u)
	}
	return unistring.String(i.s)
}

func (i *importedString) ToString() Value {
	return i
}

func (i *importedString) String() string {
	return i.s
}

func (i *importedString) ToFloat() float64 {
	i.ensureScanned()
	if i.u != nil {
		return math.NaN()
	}
	return asciiString(i.s).ToFloat()
}

func (i *importedString) ToNumber() Value {
	i.ensureScanned()
	if i.u != nil {
		return i.u.ToNumber()
	}
	return asciiString(i.s).ToNumber()
}

func (i *importedString) ToBoolean() bool {
	return len(i.s) != 0
}

func (i *importedString) ToObject(r *Runtime) *Object {
	return r._newString(i, r.global.StringPrototype)
}

func (i *importedString) SameAs(other Value) bool {
	return i.StrictEquals(other)
}

func (i *importedString) Equals(other Value) bool {
	if i.StrictEquals(other) {
		return true
	}
	i.ensureScanned()
	if i.u != nil {
		return i.u.Equals(other)
	}
	return asciiString(i.s).Equals(other)
}

func (i *importedString) StrictEquals(other Value) bool {
	switch otherStr := other.(type) {
	case asciiString:
		if i.u != nil {
			return false
		}
		return i.s == string(otherStr)
	case unicodeString:
		i.ensureScanned()
		if i.u != nil && i.u.equals(otherStr) {
			return true
		}
	case *importedString:
		return i.s == otherStr.s
	}
	return false
}

func (i *importedString) Export() interface{} {
	return i.s
}

func (i *importedString) ExportType() reflect.Type {
	return reflectTypeString
}

func (i *importedString) baseObject(r *Runtime) *Object {
	i.ensureScanned()
	if i.u != nil {
		return i.u.baseObject(r)
	}
	return asciiString(i.s).baseObject(r)
}

func (i *importedString) hash(hasher *maphash.Hash) uint64 {
	i.ensureScanned()
	if i.u != nil {
		return i.u.hash(hasher)
	}
	return asciiString(i.s).hash(hasher)
}

func (i *importedString) charAt(idx int) rune {
	i.ensureScanned()
	if i.u != nil {
		return i.u.charAt(idx)
	}
	return asciiString(i.s).charAt(idx)
}

func (i *importedString) length() int {
	i.ensureScanned()
	if i.u != nil {
		return i.u.length()
	}
	return asciiString(i.s).length()
}

func (i *importedString) concat(v valueString) valueString {
	if !i.scanned {
		if v, ok := v.(*importedString); ok {
			if !v.scanned {
				return &importedString{s: i.s + v.s}
			}
		}
		i.ensureScanned()
	}
	if i.u != nil {
		return i.u.concat(v)
	}
	return asciiString(i.s).concat(v)
}

func (i *importedString) substring(start, end int) valueString {
	i.ensureScanned()
	if i.u != nil {
		return i.u.substring(start, end)
	}
	return asciiString(i.s).substring(start, end)
}

func (i *importedString) compareTo(v valueString) int {
	return strings.Compare(i.s, v.String())
}

func (i *importedString) reader() io.RuneReader {
	if i.scanned {
		if i.u != nil {
			return i.u.reader()
		}
		return asciiString(i.s).reader()
	}
	return strings.NewReader(i.s)
}

type stringUtf16Reader struct {
	s      string
	pos    int
	second rune
}

func (s *stringUtf16Reader) ReadRune() (r rune, size int, err error) {
	if s.second >= 0 {
		r = s.second
		s.second = -1
		size = 1
		return
	}
	if s.pos < len(s.s) {
		r1, size1 := utf8.DecodeRuneInString(s.s[s.pos:])
		s.pos += size1
		size = 1
		if r1 <= 0xFFFF {
			r = r1
		} else {
			r, s.second = utf16.EncodeRune(r1)
		}
	} else {
		err = io.EOF
	}
	return
}

func (i *importedString) utf16Reader() io.RuneReader {
	if i.scanned {
		if i.u != nil {
			return i.u.utf16Reader()
		}
		return asciiString(i.s).utf16Reader()
	}
	return &stringUtf16Reader{
		s:      i.s,
		second: -1,
	}
}

func (i *importedString) utf16Runes() []rune {
	i.ensureScanned()
	if i.u != nil {
		return i.u.utf16Runes()
	}
	return asciiString(i.s).utf16Runes()
}

func (i *importedString) index(v valueString, start int) int {
	i.ensureScanned()
	if i.u != nil {
		return i.u.index(v, start)
	}
	return asciiString(i.s).index(v, start)
}

func (i *importedString) lastIndex(v valueString, pos int) int {
	i.ensureScanned()
	if i.u != nil {
		return i.u.lastIndex(v, pos)
	}
	return asciiString(i.s).lastIndex(v, pos)
}

func (i *importedString) toLower() valueString {
	i.ensureScanned()
	if i.u != nil {
		return toLower(i.s)
	}
	return asciiString(i.s).toLower()
}

func (i *importedString) toUpper() valueString {
	i.ensureScanned()
	if i.u != nil {
		caser := cases.Upper(language.Und)
		return newStringValue(caser.String(i.s))
	}
	return asciiString(i.s).toUpper()
}

func (i *importedString) toTrimmedUTF8() string {
	return strings.Trim(i.s, parser.WhitespaceChars)
}
