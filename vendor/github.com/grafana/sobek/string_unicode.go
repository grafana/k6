package sobek

import (
	"bytes"
	"errors"
	"hash/maphash"
	"io"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/grafana/sobek/unistring"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/rangetable"
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

var caseIgnorable = rangetable.Merge(
	unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk,
	// Go's unicode package doesn't expose Word_Break
	rangetable.New(
		0x0027, 0x002E, 0x003A, 0x00B7, 0x0387, 0x055F, 0x05F4,
		0x2018, 0x2019, 0x2024, 0x2027, 0xFE13, 0xFE52, 0xFE55,
		0xFF07, 0xFF0E, 0xFF1A,
	),
)

var cased = rangetable.Merge(
	unicode.Lu, unicode.Ll, unicode.Lt,
	unicode.Other_Lowercase, unicode.Other_Uppercase,
)

func isCaseIgnorable(r rune) bool { return unicode.Is(caseIgnorable, r) }
func isCased(r rune) bool         { return unicode.Is(cased, r) }

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
	if n < 0 {
		panic(rangeError("Invalid string length"))
	}
	if len(b.buf) == 0 {
		n++
	}
	if cap(b.buf)-len(b.buf) < n {
		newCap := safeMul(2, cap(b.buf)) + n
		buf := make([]uint16, len(b.buf), newCap)
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
		b.asciiBuilder.Grow(toStringLen(n))
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
	return trimmedToInteger(s.toTrimmedUTF8())
}

func (s unicodeString) toString() String {
	return s
}

func (s unicodeString) ToString() Value {
	return s
}

func (s unicodeString) ToFloat() float64 {
	return trimmedToFloat(s.toTrimmedUTF8())
}

func (s unicodeString) ToBoolean() bool {
	return len(s) > 0
}

func (s unicodeString) toTrimmedUTF8() string {
	if len(s) == 0 {
		return ""
	}
	return strings.Trim(s.String(), WhitespaceChars)
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

	if o, ok := other.(valueInt); ok {
		ss := s.toTrimmedUTF8()
		if o1, e := stringToInt(ss); e == nil {
			return o1 == int64(o)
		}
		if f, e := stringToFloat(ss); e == nil {
			return f == float64(o)
		}
		return false
	}

	if o, ok := other.(valueFloat); ok {
		return s.ToFloat() == float64(o)
	}

	if o, ok := other.(valueBool); ok {
		if o1, e := stringToFloat(s.toTrimmedUTF8()); e == nil {
			return o1 == o.ToFloat()
		}
		return false
	}

	if o, ok := other.(*valueBigInt); ok {
		bigInt, err := stringToBigInt(s.toTrimmedUTF8())
		if err != nil {
			return false
		}
		return bigInt.Cmp((*big.Int)(o)) == 0
	}

	if o, ok := other.(*Object); ok {
		return s.Equals(o.toPrimitive())
	}
	return false
}

func (s unicodeString) StrictEquals(other Value) bool {
	switch o := other.(type) {
	case unicodeString:
		return s.equals(o)
	case *importedString:
		o.ensureScanned()
		if o.u != nil {
			return s.equals(o.u)
		}
	case *concatString:
		return s.StrictEquals(o.flatten())
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

// loneSurrogateIndex returns the index of the first code unit in s that is not part of a valid surrogate pair,
// or -1 if s is a well-formed UTF-16 string.
func (s unicodeString) loneSurrogateIndex() int {
	for i := 1; i < len(s); i++ {
		c := s[i]
		if isUTF16FirstSurrogate(c) {
			if i+1 < len(s) && isUTF16SecondSurrogate(s[i+1]) {
				i++
				continue
			}
			return i - 1
		}
		if isUTF16SecondSurrogate(c) {
			return i - 1
		}
	}
	return -1
}

func (s unicodeString) Concat(other String) String {
	switch o := other.(type) {
	case asciiString:
		totalLen := s.Length() + len(o)
		if totalLen > concatThreshold {
			return &concatString{left: s, right: o, length: totalLen}
		}
		b := make([]uint16, len(s)+len(o))
		copy(b, s)
		b1 := b[len(s):]
		for i := 0; i < len(o); i++ {
			b1[i] = uint16(o[i])
		}
		return unicodeString(b)
	case unicodeString:
		totalLen := s.Length() + o.Length()
		if totalLen > concatThreshold {
			return &concatString{left: s, right: o, length: totalLen}
		}
		b := make(unicodeString, len(s)+len(o)-1)
		copy(b, s)
		copy(b[len(s):], o[1:])
		return b
	case *importedString:
		o.ensureScanned()
		if o.u != nil {
			totalLen := s.Length() + o.u.Length()
			if totalLen > concatThreshold {
				return &concatString{left: s, right: o.u, length: totalLen}
			}
			b := make(unicodeString, len(s)+len(o.u)-1)
			copy(b, s)
			copy(b[len(s):], o.u[1:])
			return b
		}
		totalLen := s.Length() + len(o.s)
		if totalLen > concatThreshold {
			return &concatString{left: s, right: asciiString(o.s), length: totalLen}
		}
		b := make([]uint16, len(s)+len(o.s))
		copy(b, s)
		b1 := b[len(s):]
		for i := 0; i < len(o.s); i++ {
			b1[i] = uint16(o.s[i])
		}
		return unicodeString(b)
	case *concatString:
		return &concatString{left: s, right: o, length: s.Length() + o.length}
	default:
		panic(unknownStringTypeErr(other))
	}
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

func caserTransform(caser cases.Caser, s string) string {
	if s == "" {
		return s
	}
	dst := make([]byte, len(s))
	src := unsafe.Slice(unsafe.StringData(s), len(s))

	for {
		nDst, _, err := caser.Transform(dst, src, true)
		if errors.Is(err, transform.ErrShortDst) {
			// Replicating the grow() logic from the text package
			m := len(dst)
			if m <= 32 {
				m = 64
			} else if m <= 256 {
				m *= 2
			} else {
				m += m >> 1
			}
			dst = make([]byte, m)
			caser.Reset()
			continue
		}
		if nDst == 0 {
			return ""
		}
		return unsafe.String(&dst[0], nDst)
	}
}

// isFinalSigma reports whether the Σ at byte offset i in s is final.
// Ignorability is tested BEFORE casedness, as per ECMAScript rules.
func isFinalSigma(s string, i int) bool {
	// Backward: skip ignorables, next rune must be cased.
	before := false
	for j := i; j > 0; {
		r, n := utf8.DecodeLastRuneInString(s[:j])
		j -= n
		if isCaseIgnorable(r) {
			continue
		}
		before = isCased(r)
		break
	}
	if !before {
		return false
	}
	// Forward: skip ignorables, next rune must NOT be cased.
	for j := i + len("Σ"); j < len(s); {
		r, n := utf8.DecodeRuneInString(s[j:])
		j += n
		if isCaseIgnorable(r) {
			continue
		}
		return !isCased(r)
	}
	return true
}

func toLower(s string, sb *StringBuilder) {
	caser := cases.Lower(language.Und)

	// Workaround for the final sigma rule
	start := 0
	for {
		k := strings.IndexRune(s[start:], 'Σ')
		if k < 0 {
			sb.WriteUTF8String(caserTransform(caser, s[start:]))
			return
		}
		k += start
		sb.WriteUTF8String(caserTransform(caser, s[start:k]))
		if isFinalSigma(s, k) {
			sb.WriteRune('ς')
		} else {
			sb.WriteRune('σ')
		}
		start = k + len("Σ")
	}
}

func toUpper(s string, sb *StringBuilder) {
	sb.WriteUTF8String(caserTransform(cases.Upper(language.Und), s))
}

// transform normalizes or maps valid Unicode spans while preserving lone UTF-16 units.
func (s unicodeString) transform(conv func(string, *StringBuilder)) String {
	var b StringBuilder
	u := s[1:]
	b.LikelyUnicode(len(u))
	start := 0
	for i := 0; i < len(u); i++ {
		c := u[i]
		if isUTF16FirstSurrogate(c) {
			if i+1 < len(u) && isUTF16SecondSurrogate(u[i+1]) {
				i++
				continue
			}
			if start < i {
				conv(string(utf16.Decode(u[start:i])), &b)
			}
			b.WriteRune(rune(c))
			start = i + 1
		} else if isUTF16SecondSurrogate(c) {
			if start < i {
				conv(string(utf16.Decode(u[start:i])), &b)
			}
			b.WriteRune(rune(c))
			start = i + 1
		}
	}
	if start < len(u) {
		conv(string(utf16.Decode(u[start:])), &b)
	}
	return b.String()
}

func (s unicodeString) toLower() String {
	return s.transform(toLower)
}

func (s unicodeString) toUpper() String {
	return s.transform(toUpper)
}

func (s unicodeString) normalize(form norm.Form) String {
	return s.transform(func(span string, b *StringBuilder) {
		b.WriteUTF8String(form.String(span))
	})
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
