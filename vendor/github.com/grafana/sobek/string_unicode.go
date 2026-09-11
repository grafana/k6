package sobek

import (
	"bytes"
	"errors"
	"hash/maphash"
	"io"
	"math"
	"reflect"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/grafana/sobek/parser"
	"github.com/grafana/sobek/unistring"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const utf16IndexNaiveCrossover = 32

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
	utf16Reader utf16Reader
	prev        uint16
	prevSet     bool
}

// StringBuilder serves similar purpose to strings.Builder, except it works with ECMAScript String.
// Use it to efficiently build 'native' ECMAScript values that either contain invalid UTF-16 surrogate pairs
// (and therefore cannot be represented as UTF-8) or never expected to be exported to Go. See also
// StringFromUTF16.
type StringBuilder struct {
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

func (rr *utf16RuneReader) readChar() (c uint16, err error) {
	if rr.pos < len(rr.s) {
		c = rr.s[rr.pos]
		rr.pos++
		return
	}
	err = io.EOF
	return
}

func (rr *utf16RuneReader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		r = rune(rr.s[rr.pos])
		rr.pos++
		size = 1
		return
	}
	err = io.EOF
	return
}

func (rr *lenientUtf16Decoder) ReadRune() (r rune, size int, err error) {
	var c uint16
	if rr.prevSet {
		c = rr.prev
		rr.prevSet = false
	} else {
		c, err = rr.utf16Reader.readChar()
		if err != nil {
			return
		}
	}
	size = 1
	if isUTF16FirstSurrogate(c) {
		second, err1 := rr.utf16Reader.readChar()
		if err1 != nil {
			if err1 != io.EOF {
				err = err1
			} else {
				r = rune(c)
			}
			return
		}
		if isUTF16SecondSurrogate(second) {
			r = utf16.DecodeRune(rune(c), rune(second))
			size++
			return
		} else {
			rr.prev = second
			rr.prevSet = true
		}
	}
	r = rune(c)
	return
}

func (rr *unicodeRuneReader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		c := rr.s[rr.pos]
		size++
		rr.pos++
		if isUTF16FirstSurrogate(c) {
			if rr.pos < len(rr.s) {
				second := rr.s[rr.pos]
				if isUTF16SecondSurrogate(second) {
					r = utf16.DecodeRune(rune(c), rune(second))
					size++
					rr.pos++
					return
				}
			}
			err = InvalidRuneError
		} else if isUTF16SecondSurrogate(c) {
			err = InvalidRuneError
		}
		r = rune(c)
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
	if len(b.buf) == 0 {
		b.buf = append(b.buf, unistring.BOM)
	}
}

// assumes already started
func (b *unicodeStringBuilder) writeString(s String) {
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

func (b *unicodeStringBuilder) String() String {
	if b.unicode {
		return unicodeString(b.buf)
	}
	if len(b.buf) < 2 {
		return stringEmpty
	}
	buf := make([]byte, 0, len(b.buf)-1)
	for _, c := range b.buf[1:] {
		buf = append(buf, byte(c))
	}
	return asciiString(buf)
}

func (b *unicodeStringBuilder) WriteRune(r rune) {
	b.Grow(2)
	b.writeRuneFast(r)
}

// assumes already started
func (b *unicodeStringBuilder) writeRuneFast(r rune) {
	if r <= 0xFFFF {
		b.buf = append(b.buf, uint16(r))
		if !b.unicode && r >= utf8.RuneSelf {
			b.unicode = true
		}
	} else {
		first, second := utf16.EncodeRune(r)
		b.buf = append(b.buf, uint16(first), uint16(second))
		b.unicode = true
	}
}

func (b *unicodeStringBuilder) writeASCIIString(bytes string) {
	for _, c := range bytes {
		b.buf = append(b.buf, uint16(c))
	}
}

func (b *unicodeStringBuilder) writeUnicodeString(str unicodeString) {
	b.buf = append(b.buf, str[1:]...)
	b.unicode = true
}

func (b *StringBuilder) ascii() bool {
	return len(b.unicodeBuilder.buf) == 0
}

func (b *StringBuilder) WriteString(s String) {
	a, u := devirtualizeString(s)
	if u != nil {
		b.switchToUnicode(u.Length())
		b.unicodeBuilder.writeUnicodeString(u)
	} else {
		if b.ascii() {
			b.asciiBuilder.WriteString(string(a))
		} else {
			b.unicodeBuilder.writeASCIIString(string(a))
		}
	}
}

func (b *StringBuilder) WriteUTF8String(s string) {
	firstUnicodeIdx := 0
	if b.ascii() {
		for i := 0; i < len(s); i++ {
			if s[i] >= utf8.RuneSelf {
				b.switchToUnicode(len(s))
				b.unicodeBuilder.writeASCIIString(s[:i])
				firstUnicodeIdx = i
				goto unicode
			}
		}
		b.asciiBuilder.WriteString(s)
		return
	}
unicode:
	for _, r := range s[firstUnicodeIdx:] {
		b.unicodeBuilder.writeRuneFast(r)
	}
}

func (b *StringBuilder) writeASCII(s string) {
	if b.ascii() {
		b.asciiBuilder.WriteString(s)
	} else {
		b.unicodeBuilder.writeASCIIString(s)
	}
}

func (b *StringBuilder) WriteRune(r rune) {
	if r < utf8.RuneSelf {
		if b.ascii() {
			b.asciiBuilder.WriteByte(byte(r))
		} else {
			b.unicodeBuilder.writeRuneFast(r)
		}
	} else {
		var extraLen int
		if r <= 0xFFFF {
			extraLen = 1
		} else {
			extraLen = 2
		}
		b.switchToUnicode(extraLen)
		b.unicodeBuilder.writeRuneFast(r)
	}
}

func (b *StringBuilder) String() String {
	if b.ascii() {
		return asciiString(b.asciiBuilder.String())
	}
	return b.unicodeBuilder.String()
}

func (b *StringBuilder) Grow(n int) {
	if b.ascii() {
		b.asciiBuilder.Grow(n)
	} else {
		b.unicodeBuilder.Grow(n)
	}
}

// LikelyUnicode hints to the builder that the resulting string is likely to contain Unicode (non-ASCII) characters.
// The argument is an extra capacity (in characters) to reserve on top of the current length (it's like calling
// Grow() afterwards).
// This method may be called at any point (not just when the buffer is empty), although for efficiency it should
// be called as early as possible.
func (b *StringBuilder) LikelyUnicode(extraLen int) {
	b.switchToUnicode(extraLen)
}

func (b *StringBuilder) switchToUnicode(extraLen int) {
	if b.ascii() {
		c := b.asciiBuilder.Cap()
		newCap := b.asciiBuilder.Len() + extraLen
		if newCap < c {
			newCap = c
		}
		b.unicodeBuilder.Grow(newCap)
		b.unicodeBuilder.writeASCIIString(b.asciiBuilder.String())
		b.asciiBuilder.Reset()
	}
}

func (b *StringBuilder) WriteSubstring(source String, start int, end int) {
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
			if us.CharAt(i) >= utf8.RuneSelf {
				uc = true
				break
			}
		}
		if uc {
			b.switchToUnicode(end - start + 1)
		} else {
			b.asciiBuilder.Grow(end - start + 1)
			for i := start; i < end; i++ {
				b.asciiBuilder.WriteByte(byte(us.CharAt(i)))
			}
			return
		}
	}
	b.unicodeBuilder.buf = append(b.unicodeBuilder.buf, us[start+1:end+1]...)
	b.unicodeBuilder.unicode = true
}

func (s unicodeString) Reader() io.RuneReader {
	return &unicodeRuneReader{
		s: s[1:],
	}
}

func (s unicodeString) utf16Reader() utf16Reader {
	return &utf16RuneReader{
		s: s[1:],
	}
}

func (s unicodeString) utf16RuneReader() io.RuneReader {
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

func (s unicodeString) toString() String {
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
	return r._newString(s, r.getStringPrototype())
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
	ss := r.getStringSingleton()
	ss.value = s
	ss.setLength()
	return ss.val
}

func (s unicodeString) CharAt(idx int) uint16 {
	return s[idx+1]
}

func (s unicodeString) Length() int {
	return len(s) - 1
}

func (s unicodeString) Concat(other String) String {
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

func (s unicodeString) Substring(start, end int) String {
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

func (s unicodeString) compareToAscii(s2 asciiString) int {
	s1 := s[1:]
	for i := range s1 {
		if i >= len(s2) {
			return 1
		}
		c1 := s1[i]
		c2 := uint16(s2[i])
		if c1 < c2 {
			return -1
		}
		if c1 > c2 {
			return 1
		}
	}
	// This condition will never be true as long as unicodeString contains at least one
	// Unicode character, but is left here for completeness.
	if len(s2) > len(s1) {
		return -1
	}
	return 0
}

func (s unicodeString) CompareTo(other String) int {
	oa, ou := devirtualizeString(other)
	if ou != nil {
		return slices.Compare(s[1:], ou[1:])
	}
	return s.compareToAscii(oa)
}

func utf16Index(s, substr []uint16) int {
	if len(s) < utf16IndexNaiveCrossover {
		return utf16IndexNaive(s, substr)
	}
	return utf16IndexBytes(s, substr)
}

func utf16IndexNaive(s, sep []uint16) int {
	n := len(sep)
	if n == 0 {
		return 0
	}
	if n > len(s) {
		return -1
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i] == sep[0] {
			match := true
			for j := 1; j < n; j++ {
				if s[i+j] != sep[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}

func uint16AsBytes(s []uint16) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*2)
}

func utf16IndexBytes(s, sep []uint16) int {
	if len(sep) == 0 {
		return 0
	}
	if len(sep) > len(s) {
		return -1
	}

	sb := uint16AsBytes(s)
	sepb := uint16AsBytes(sep)

	offset := 0
	for {
		i := bytes.Index(sb[offset:], sepb)
		if i < 0 {
			return -1
		}
		pos := offset + i
		if pos%2 == 0 {
			return pos / 2
		}
		// False match, not aligned, continue
		offset = pos + 1
	}
}

func (s unicodeString) index(substr String, start int) int {
	var ss []uint16
	a, u := devirtualizeString(substr)
	if u != nil {
		ss = u[1:]
	} else {
		ss = a.utf16()
	}
	idx := utf16Index(s[min(1+start, len(s)):], ss)
	if idx != -1 {
		return idx + start
	}
	return -1
}

func utf16LastIndex(s, sep []uint16) int {
	if len(s) < utf16IndexNaiveCrossover {
		return utf16LastIndexNaive(s, sep)
	}
	return utf16LastIndexBytes(s, sep)
}

func utf16LastIndexNaive(s, sep []uint16) int {
	n := len(sep)
	if n == 0 {
		return len(s)
	}
	if n > len(s) {
		return -1
	}
	for i := len(s) - n; i >= 0; i-- {
		if s[i] == sep[0] {
			match := true
			for j := 1; j < n; j++ {
				if s[i+j] != sep[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}

func utf16LastIndexBytes(s, sep []uint16) int {
	if len(sep) == 0 {
		return len(s)
	}
	if len(sep) > len(s) {
		return -1
	}

	sb := uint16AsBytes(s)
	sepb := uint16AsBytes(sep)

	end := len(sb)
	for end >= len(sepb) {
		i := bytes.LastIndex(sb[:end], sepb)
		if i < 0 {
			return -1
		}
		if i%2 == 0 {
			return i / 2
		}
		// False match starting at odd byte offset i. Shrink the window so
		// the next search can still find a match starting at i-1 (i.e.
		// end must be >= (i-1)+len(sepb)) while excluding this same match
		// (end must be < i+len(sepb)). end = i+len(sepb)-1 satisfies both.
		end = i + len(sepb) - 1
	}
	return -1
}

func (s unicodeString) lastIndex(substr String, start int) int {
	var ss []uint16
	a, u := devirtualizeString(substr)
	if u != nil {
		ss = u[1:]
	} else {
		ss = a.utf16()
	}
	return utf16LastIndex(s[1:min(start+1+len(ss), len(s))], ss)
}

func unicodeStringFromRunes(r []rune) unicodeString {
	return unistring.NewFromRunes(r).AsUtf16()
}

func toLower(s string) String {
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

func (s unicodeString) toLower() String {
	return toLower(s.String())
}

func (s unicodeString) toUpper() String {
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
