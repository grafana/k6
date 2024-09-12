package sobek

import (
	"hash/maphash"
	"io"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/grafana/sobek/unistring"
)

type asciiString string

type asciiRuneReader struct {
	s   asciiString
	pos int
}

func (rr *asciiRuneReader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		r = rune(rr.s[rr.pos])
		size = 1
		rr.pos++
	} else {
		err = io.EOF
	}
	return
}

type asciiUtf16Reader struct {
	s   asciiString
	pos int
}

func (rr *asciiUtf16Reader) readChar() (c uint16, err error) {
	if rr.pos < len(rr.s) {
		c = uint16(rr.s[rr.pos])
		rr.pos++
	} else {
		err = io.EOF
	}
	return
}

func (rr *asciiUtf16Reader) ReadRune() (r rune, size int, err error) {
	if rr.pos < len(rr.s) {
		r = rune(rr.s[rr.pos])
		rr.pos++
		size = 1
	} else {
		err = io.EOF
	}
	return
}

func (s asciiString) Reader() io.RuneReader {
	return &asciiRuneReader{
		s: s,
	}
}

func (s asciiString) utf16Reader() utf16Reader {
	return &asciiUtf16Reader{
		s: s,
	}
}

func (s asciiString) utf16RuneReader() io.RuneReader {
	return &asciiUtf16Reader{
		s: s,
	}
}

func (s asciiString) utf16Runes() []rune {
	runes := make([]rune, len(s))
	for i := 0; i < len(s); i++ {
		runes[i] = rune(s[i])
	}
	return runes
}

// ss must be trimmed
func stringToInt(ss string) (int64, error) {
	if ss == "" {
		return 0, nil
	}
	if ss == "-0" {
		return 0, strconv.ErrSyntax
	}
	if len(ss) > 2 {
		switch ss[:2] {
		case "0x", "0X":
			return strconv.ParseInt(ss[2:], 16, 64)
		case "0b", "0B":
			return strconv.ParseInt(ss[2:], 2, 64)
		case "0o", "0O":
			return strconv.ParseInt(ss[2:], 8, 64)
		}
	}
	return strconv.ParseInt(ss, 10, 64)
}

func (s asciiString) _toInt(trimmed string) (int64, error) {
	return stringToInt(trimmed)
}

func isRangeErr(err error) bool {
	if err, ok := err.(*strconv.NumError); ok {
		return err.Err == strconv.ErrRange
	}
	return false
}

func (s asciiString) _toFloat(trimmed string) (float64, error) {
	if trimmed == "" {
		return 0, nil
	}
	if trimmed == "-0" {
		var f float64
		return -f, nil
	}

	// Go allows underscores in numbers, when parsed as floats, but ECMAScript expect them to be interpreted as NaN.
	if strings.ContainsRune(trimmed, '_') {
		return 0, strconv.ErrSyntax
	}

	// Hexadecimal floats are not supported by ECMAScript.
	if len(trimmed) >= 2 {
		var prefix string
		if trimmed[0] == '-' || trimmed[0] == '+' {
			prefix = trimmed[1:]
		} else {
			prefix = trimmed
		}
		if len(prefix) >= 2 && prefix[0] == '0' && (prefix[1] == 'x' || prefix[1] == 'X') {
			return 0, strconv.ErrSyntax
		}
	}

	f, err := strconv.ParseFloat(trimmed, 64)
	if err == nil && math.IsInf(f, 0) {
		ss := strings.ToLower(trimmed)
		if strings.HasPrefix(ss, "inf") || strings.HasPrefix(ss, "-inf") || strings.HasPrefix(ss, "+inf") {
			// We handle "Infinity" separately, prevent from being parsed as Infinity due to strconv.ParseFloat() permissive syntax
			return 0, strconv.ErrSyntax
		}
	}
	if isRangeErr(err) {
		err = nil
	}
	return f, err
}

func (s asciiString) ToInteger() int64 {
	ss := strings.TrimSpace(string(s))
	if ss == "" {
		return 0
	}
	if ss == "Infinity" || ss == "+Infinity" {
		return math.MaxInt64
	}
	if ss == "-Infinity" {
		return math.MinInt64
	}
	i, err := s._toInt(ss)
	if err != nil {
		f, err := s._toFloat(ss)
		if err == nil {
			return int64(f)
		}
	}
	return i
}

func (s asciiString) toString() String {
	return s
}

func (s asciiString) ToString() Value {
	return s
}

func (s asciiString) String() string {
	return string(s)
}

func (s asciiString) ToFloat() float64 {
	ss := strings.TrimSpace(string(s))
	if ss == "" {
		return 0
	}
	if ss == "Infinity" || ss == "+Infinity" {
		return math.Inf(1)
	}
	if ss == "-Infinity" {
		return math.Inf(-1)
	}
	f, err := s._toFloat(ss)
	if err != nil {
		i, err := s._toInt(ss)
		if err == nil {
			return float64(i)
		}
		f = math.NaN()
	}
	return f
}

func (s asciiString) ToBoolean() bool {
	return s != ""
}

func (s asciiString) ToNumber() Value {
	ss := strings.TrimSpace(string(s))
	if ss == "" {
		return intToValue(0)
	}
	if ss == "Infinity" || ss == "+Infinity" {
		return _positiveInf
	}
	if ss == "-Infinity" {
		return _negativeInf
	}

	if i, err := s._toInt(ss); err == nil {
		return intToValue(i)
	}

	if f, err := s._toFloat(ss); err == nil {
		return floatToValue(f)
	}

	return _NaN
}

func (s asciiString) ToObject(r *Runtime) *Object {
	return r._newString(s, r.getStringPrototype())
}

func (s asciiString) SameAs(other Value) bool {
	return s.StrictEquals(other)
}

func (s asciiString) Equals(other Value) bool {
	if s.StrictEquals(other) {
		return true
	}

	if o, ok := other.(valueInt); ok {
		if o1, e := s._toInt(strings.TrimSpace(string(s))); e == nil {
			return o1 == int64(o)
		}
		return false
	}

	if o, ok := other.(valueFloat); ok {
		return s.ToFloat() == float64(o)
	}

	if o, ok := other.(valueBool); ok {
		if o1, e := s._toFloat(strings.TrimSpace(string(s))); e == nil {
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

func (s asciiString) StrictEquals(other Value) bool {
	if otherStr, ok := other.(asciiString); ok {
		return s == otherStr
	}
	if otherStr, ok := other.(*importedString); ok {
		if otherStr.u == nil {
			return string(s) == otherStr.s
		}
	}
	return false
}

func (s asciiString) baseObject(r *Runtime) *Object {
	ss := r.getStringSingleton()
	ss.value = s
	ss.setLength()
	return ss.val
}

func (s asciiString) hash(hash *maphash.Hash) uint64 {
	_, _ = hash.WriteString(string(s))
	h := hash.Sum64()
	hash.Reset()
	return h
}

func (s asciiString) CharAt(idx int) uint16 {
	return uint16(s[idx])
}

func (s asciiString) Length() int {
	return len(s)
}

func (s asciiString) Concat(other String) String {
	a, u := devirtualizeString(other)
	if u != nil {
		b := make([]uint16, len(s)+len(u))
		b[0] = unistring.BOM
		for i := 0; i < len(s); i++ {
			b[i+1] = uint16(s[i])
		}
		copy(b[len(s)+1:], u[1:])
		return unicodeString(b)
	}
	return s + a
}

func (s asciiString) Substring(start, end int) String {
	return s[start:end]
}

func (s asciiString) CompareTo(other String) int {
	switch other := other.(type) {
	case asciiString:
		return strings.Compare(string(s), string(other))
	case unicodeString:
		return strings.Compare(string(s), other.String())
	case *importedString:
		return strings.Compare(string(s), other.s)
	default:
		panic(newTypeError("Internal bug: unknown string type: %T", other))
	}
}

func (s asciiString) index(substr String, start int) int {
	a, u := devirtualizeString(substr)
	if u == nil {
		if start > len(s) {
			return -1
		}
		p := strings.Index(string(s[start:]), string(a))
		if p >= 0 {
			return p + start
		}
	}
	return -1
}

func (s asciiString) lastIndex(substr String, pos int) int {
	a, u := devirtualizeString(substr)
	if u == nil {
		end := pos + len(a)
		var ss string
		if end > len(s) {
			ss = string(s)
		} else {
			ss = string(s[:end])
		}
		return strings.LastIndex(ss, string(a))
	}
	return -1
}

func (s asciiString) toLower() String {
	return asciiString(strings.ToLower(string(s)))
}

func (s asciiString) toUpper() String {
	return asciiString(strings.ToUpper(string(s)))
}

func (s asciiString) toTrimmedUTF8() string {
	return strings.TrimSpace(string(s))
}

func (s asciiString) string() unistring.String {
	return unistring.String(s)
}

func (s asciiString) Export() interface{} {
	return string(s)
}

func (s asciiString) ExportType() reflect.Type {
	return reflectTypeString
}
