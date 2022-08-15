package goja

import (
	"errors"
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

type unicodeString []uint16

type unicodeRuneReader struct {
	s   unicodeString
	pos int
}

type utf16RuneReader struct {
	s   unicodeString
	pos int
}

// passes through invalid surrogate pairs
type lenientUtf16Decoder struct {
	utf16Reader io.RuneReader
	prev        rune
	prevSet     bool
}

type valueStringBuilder struct {
	asciiBuilder   strings.Builder
	unicodeBuilder unicodeStringBuilder
}

type unicodeStringBuilder struct {
	buf     []uint16
	unicode bool
}

var (
	InvalidRuneError = errors.New("invalid rune")
)

func (rr *utf16RuneReader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		r = rune(rr.s[rr.pos])
		size++
		rr.pos++
		return
	}
	err = io.EOF
	return
}

func (rr *lenientUtf16Decoder) ReadRune() (r rune, size int, err error) {
	if rr.prevSet {
		r = rr.prev
		size = 1
		rr.prevSet = false
	} else {
		r, size, err = rr.utf16Reader.ReadRune()
		if err != nil {
			return
		}
	}
	if isUTF16FirstSurrogate(r) {
		second, _, err1 := rr.utf16Reader.ReadRune()
		if err1 != nil {
			if err1 != io.EOF {
				err = err1
			}
			return
		}
		if isUTF16SecondSurrogate(second) {
			r = utf16.DecodeRune(r, second)
			size++
		} else {
			rr.prev = second
			rr.prevSet = true
		}
	}

	return
}

func (rr *unicodeRuneReader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		r = rune(rr.s[rr.pos])
		size++
		rr.pos++
		if isUTF16FirstSurrogate(r) {
			if rr.pos < len(rr.s) {
				second := rune(rr.s[rr.pos])
				if isUTF16SecondSurrogate(second) {
					r = utf16.DecodeRune(r, second)
					size++
					rr.pos++
				} else {
					err = InvalidRuneError
				}
			} else {
				err = InvalidRuneError
			}
		} else if isUTF16SecondSurrogate(r) {
			err = InvalidRuneError
		}
	} else {
		err = io.EOF
	}
	return
}

func (b *unicodeStringBuilder) Grow(n int) {
	if len(b.buf) == 0 {
		n++
	}
	if cap(b.buf)-len(b.buf) < n {
		buf := make([]uint16, len(b.buf), 2*cap(b.buf)+n)
		copy(buf, b.buf)
		b.buf = buf
	}
}

func (b *unicodeStringBuilder) ensureStarted(initialSize int) {
	b.Grow(initialSize)
	if len(b.buf) == 0 {
		b.buf = append(b.buf, unistring.BOM)
	}
}

func (b *unicodeStringBuilder) WriteString(s valueString) {
	b.ensureStarted(s.length())
	a, u := devirtualizeString(s)
	if u != nil {
		b.buf = append(b.buf, u[1:]...)
		b.unicode = true
	} else {
		for i := 0; i < len(a); i++ {
			b.buf = append(b.buf, uint16(a[i]))
		}
	}
}

func (b *unicodeStringBuilder) String() valueString {
	if b.unicode {
		return unicodeString(b.buf)
	}
	if len(b.buf) == 0 {
		return stringEmpty
	}
	buf := make([]byte, 0, len(b.buf)-1)
	for _, c := range b.buf[1:] {
		buf = append(buf, byte(c))
	}
	return asciiString(buf)
}

func (b *unicodeStringBuilder) WriteRune(r rune) {
	if r <= 0xFFFF {
		b.ensureStarted(1)
		b.buf = append(b.buf, uint16(r))
		if !b.unicode && r >= utf8.RuneSelf {
			b.unicode = true
		}
	} else {
		b.ensureStarted(2)
		first, second := utf16.EncodeRune(r)
		b.buf = append(b.buf, uint16(first), uint16(second))
		b.unicode = true
	}
}

func (b *unicodeStringBuilder) writeASCIIString(bytes string) {
	b.ensureStarted(len(bytes))
	for _, c := range bytes {
		b.buf = append(b.buf, uint16(c))
	}
}

func (b *unicodeStringBuilder) writeUnicodeString(str unicodeString) {
	b.ensureStarted(str.length())
	b.buf = append(b.buf, str[1:]...)
	b.unicode = true
}

func (b *valueStringBuilder) ascii() bool {
	return len(b.unicodeBuilder.buf) == 0
}

func (b *valueStringBuilder) WriteString(s valueString) {
	a, u := devirtualizeString(s)
	if u != nil {
		b.switchToUnicode(u.length())
		b.unicodeBuilder.writeUnicodeString(u)
	} else {
		if b.ascii() {
			b.asciiBuilder.WriteString(string(a))
		} else {
			b.unicodeBuilder.writeASCIIString(string(a))
		}
	}
}

func (b *valueStringBuilder) WriteASCII(s string) {
	if b.ascii() {
		b.asciiBuilder.WriteString(s)
	} else {
		b.unicodeBuilder.writeASCIIString(s)
	}
}

func (b *valueStringBuilder) WriteRune(r rune) {
	if r < utf8.RuneSelf {
		if b.ascii() {
			b.asciiBuilder.WriteByte(byte(r))
		} else {
			b.unicodeBuilder.WriteRune(r)
		}
	} else {
		var extraLen int
		if r <= 0xFFFF {
			extraLen = 1
		} else {
			extraLen = 2
		}
		b.switchToUnicode(extraLen)
		b.unicodeBuilder.WriteRune(r)
	}
}

func (b *valueStringBuilder) String() valueString {
	if b.ascii() {
		return asciiString(b.asciiBuilder.String())
	}
	return b.unicodeBuilder.String()
}

func (b *valueStringBuilder) Grow(n int) {
	if b.ascii() {
		b.asciiBuilder.Grow(n)
	} else {
		b.unicodeBuilder.Grow(n)
	}
}

func (b *valueStringBuilder) switchToUnicode(extraLen int) {
	if b.ascii() {
		b.unicodeBuilder.ensureStarted(b.asciiBuilder.Len() + extraLen)
		b.unicodeBuilder.writeASCIIString(b.asciiBuilder.String())
		b.asciiBuilder.Reset()
	}
}

func (b *valueStringBuilder) WriteSubstring(source valueString, start int, end int) {
	a, us := devirtualizeString(source)
	if us == nil {
		if b.ascii() {
			b.asciiBuilder.WriteString(string(a[start:end]))
		} else {
			b.unicodeBuilder.writeASCIIString(string(a[start:end]))
		}
		return
	}
	if b.ascii() {
		uc := false
		for i := start; i < end; i++ {
			if us.charAt(i) >= utf8.RuneSelf {
				uc = true
				break
			}
		}
		if uc {
			b.switchToUnicode(end - start + 1)
		} else {
			b.asciiBuilder.Grow(end - start + 1)
			for i := start; i < end; i++ {
				b.asciiBuilder.WriteByte(byte(us.charAt(i)))
			}
			return
		}
	}
	b.unicodeBuilder.buf = append(b.unicodeBuilder.buf, us[start+1:end+1]...)
	b.unicodeBuilder.unicode = true
}

func (s unicodeString) reader() io.RuneReader {
	return &unicodeRuneReader{
		s: s[1:],
	}
}

func (s unicodeString) utf16Reader() io.RuneReader {
	return &utf16RuneReader{
		s: s[1:],
	}
}

func (s unicodeString) utf16Runes() []rune {
	runes := make([]rune, len(s)-1)
	for i, ch := range s[1:] {
		runes[i] = rune(ch)
	}
	return runes
}

func (s unicodeString) ToInteger() int64 {
	return 0
}

func (s unicodeString) toString() valueString {
	return s
}

func (s unicodeString) ToString() Value {
	return s
}

func (s unicodeString) ToFloat() float64 {
	return math.NaN()
}

func (s unicodeString) ToBoolean() bool {
	return len(s) > 0
}

func (s unicodeString) toTrimmedUTF8() string {
	if len(s) == 0 {
		return ""
	}
	return strings.Trim(s.String(), parser.WhitespaceChars)
}

func (s unicodeString) ToNumber() Value {
	return asciiString(s.toTrimmedUTF8()).ToNumber()
}

func (s unicodeString) ToObject(r *Runtime) *Object {
	return r._newString(s, r.global.StringPrototype)
}

func (s unicodeString) equals(other unicodeString) bool {
	if len(s) != len(other) {
		return false
	}
	for i, r := range s {
		if r != other[i] {
			return false
		}
	}
	return true
}

func (s unicodeString) SameAs(other Value) bool {
	return s.StrictEquals(other)
}

func (s unicodeString) Equals(other Value) bool {
	if s.StrictEquals(other) {
		return true
	}

	if o, ok := other.(*Object); ok {
		return s.Equals(o.toPrimitive())
	}
	return false
}

func (s unicodeString) StrictEquals(other Value) bool {
	if otherStr, ok := other.(unicodeString); ok {
		return s.equals(otherStr)
	}
	if otherStr, ok := other.(*importedString); ok {
		otherStr.ensureScanned()
		if otherStr.u != nil {
			return s.equals(otherStr.u)
		}
	}

	return false
}

func (s unicodeString) baseObject(r *Runtime) *Object {
	ss := r.stringSingleton
	ss.value = s
	ss.setLength()
	return ss.val
}

func (s unicodeString) charAt(idx int) rune {
	return rune(s[idx+1])
}

func (s unicodeString) length() int {
	return len(s) - 1
}

func (s unicodeString) concat(other valueString) valueString {
	a, u := devirtualizeString(other)
	if u != nil {
		b := make(unicodeString, len(s)+len(u)-1)
		copy(b, s)
		copy(b[len(s):], u[1:])
		return b
	}
	b := make([]uint16, len(s)+len(a))
	copy(b, s)
	b1 := b[len(s):]
	for i := 0; i < len(a); i++ {
		b1[i] = uint16(a[i])
	}
	return unicodeString(b)
}

func (s unicodeString) substring(start, end int) valueString {
	ss := s[start+1 : end+1]
	for _, c := range ss {
		if c >= utf8.RuneSelf {
			b := make(unicodeString, end-start+1)
			b[0] = unistring.BOM
			copy(b[1:], ss)
			return b
		}
	}
	as := make([]byte, end-start)
	for i, c := range ss {
		as[i] = byte(c)
	}
	return asciiString(as)
}

func (s unicodeString) String() string {
	return string(utf16.Decode(s[1:]))
}

func (s unicodeString) compareTo(other valueString) int {
	// TODO handle invalid UTF-16
	return strings.Compare(s.String(), other.String())
}

func (s unicodeString) index(substr valueString, start int) int {
	var ss []uint16
	a, u := devirtualizeString(substr)
	if u != nil {
		ss = u[1:]
	} else {
		ss = make([]uint16, len(a))
		for i := 0; i < len(a); i++ {
			ss[i] = uint16(a[i])
		}
	}
	s1 := s[1:]
	// TODO: optimise
	end := len(s1) - len(ss)
	for start <= end {
		for i := 0; i < len(ss); i++ {
			if s1[start+i] != ss[i] {
				goto nomatch
			}
		}

		return start
	nomatch:
		start++
	}
	return -1
}

func (s unicodeString) lastIndex(substr valueString, start int) int {
	var ss []uint16
	a, u := devirtualizeString(substr)
	if u != nil {
		ss = u[1:]
	} else {
		ss = make([]uint16, len(a))
		for i := 0; i < len(a); i++ {
			ss[i] = uint16(a[i])
		}
	}

	s1 := s[1:]
	if maxStart := len(s1) - len(ss); start > maxStart {
		start = maxStart
	}
	// TODO: optimise
	for start >= 0 {
		for i := 0; i < len(ss); i++ {
			if s1[start+i] != ss[i] {
				goto nomatch
			}
		}

		return start
	nomatch:
		start--
	}
	return -1
}

func unicodeStringFromRunes(r []rune) unicodeString {
	return unistring.NewFromRunes(r).AsUtf16()
}

func toLower(s string) valueString {
	caser := cases.Lower(language.Und)
	r := []rune(caser.String(s))
	// Workaround
	ascii := true
	for i := 0; i < len(r)-1; i++ {
		if (i == 0 || r[i-1] != 0x3b1) && r[i] == 0x345 && r[i+1] == 0x3c2 {
			i++
			r[i] = 0x3c3
		}
		if r[i] >= utf8.RuneSelf {
			ascii = false
		}
	}
	if ascii {
		ascii = r[len(r)-1] < utf8.RuneSelf
	}
	if ascii {
		return asciiString(r)
	}
	return unicodeStringFromRunes(r)
}

func (s unicodeString) toLower() valueString {
	return toLower(s.String())
}

func (s unicodeString) toUpper() valueString {
	caser := cases.Upper(language.Und)
	return newStringValue(caser.String(s.String()))
}

func (s unicodeString) Export() interface{} {
	return s.String()
}

func (s unicodeString) ExportType() reflect.Type {
	return reflectTypeString
}

func (s unicodeString) hash(hash *maphash.Hash) uint64 {
	_, _ = hash.WriteString(string(unistring.FromUtf16(s)))
	h := hash.Sum64()
	hash.Reset()
	return h
}

func (s unicodeString) string() unistring.String {
	return unistring.FromUtf16(s)
}
