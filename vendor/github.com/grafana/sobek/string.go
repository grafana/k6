package sobek

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/grafana/sobek/unistring"
)

const (
	__proto__ = "__proto__"
)

var (
	stringTrue        String = asciiString("true")
	stringFalse       String = asciiString("false")
	stringNull        String = asciiString("null")
	stringUndefined   String = asciiString("undefined")
	stringObjectC     String = asciiString("object")
	stringFunction    String = asciiString("function")
	stringBoolean     String = asciiString("boolean")
	stringString      String = asciiString("string")
	stringSymbol      String = asciiString("symbol")
	stringNumber      String = asciiString("number")
	stringBigInt      String = asciiString("bigint")
	stringNaN         String = asciiString("NaN")
	stringInfinity           = asciiString("Infinity")
	stringNegInfinity        = asciiString("-Infinity")
	stringBound_      String = asciiString("bound ")
	stringEmpty       String = asciiString("")

	stringError          String = asciiString("Error")
	stringAggregateError String = asciiString("AggregateError")
	stringTypeError      String = asciiString("TypeError")
	stringReferenceError String = asciiString("ReferenceError")
	stringSyntaxError    String = asciiString("SyntaxError")
	stringRangeError     String = asciiString("RangeError")
	stringEvalError      String = asciiString("EvalError")
	stringURIError       String = asciiString("URIError")
	stringGoError        String = asciiString("GoError")

	stringObjectNull      String = asciiString("[object Null]")
	stringObjectUndefined String = asciiString("[object Undefined]")
	stringInvalidDate     String = asciiString("Invalid Date")
)

type utf16Reader interface {
	readChar() (c uint16, err error)
}

// String represents an ECMAScript string Value. Its internal representation depends on the contents of the
// string, but in any case it is capable of holding any UTF-16 string, either valid or invalid.
// Instances of this type, as any other primitive values, are goroutine-safe and can be passed between runtimes.
// Strings can be created using Runtime.ToValue(goString) or StringFromUTF16.
type String interface {
	Value
	CharAt(int) uint16
	Length() int
	Concat(String) String
	Substring(start, end int) String
	CompareTo(String) int
	Reader() io.RuneReader
	utf16Reader() utf16Reader
	utf16RuneReader() io.RuneReader
	utf16Runes() []rune
	index(String, int) int
	lastIndex(String, int) int
	toLower() String
	toUpper() String
	toTrimmedUTF8() string
}

type stringIterObject struct {
	baseObject
	reader io.RuneReader
}

func isUTF16FirstSurrogate(c uint16) bool {
	return c >= 0xD800 && c <= 0xDBFF
}

func isUTF16SecondSurrogate(c uint16) bool {
	return c >= 0xDC00 && c <= 0xDFFF
}

func (si *stringIterObject) next() Value {
	if si.reader == nil {
		return si.val.runtime.createIterResultObject(_undefined, true)
	}
	r, _, err := si.reader.ReadRune()
	if err == io.EOF {
		si.reader = nil
		return si.val.runtime.createIterResultObject(_undefined, true)
	}
	return si.val.runtime.createIterResultObject(stringFromRune(r), false)
}

func stringFromRune(r rune) String {
	if r < utf8.RuneSelf {
		var sb strings.Builder
		sb.WriteByte(byte(r))
		return asciiString(sb.String())
	}
	var sb unicodeStringBuilder
	sb.WriteRune(r)
	return sb.String()
}

func (r *Runtime) createStringIterator(s String) Value {
	o := &Object{runtime: r}

	si := &stringIterObject{
		reader: &lenientUtf16Decoder{utf16Reader: s.utf16Reader()},
	}
	si.class = classObject
	si.val = o
	si.extensible = true
	o.self = si
	si.prototype = r.getStringIteratorPrototype()
	si.init()

	return o
}

type stringObject struct {
	baseObject
	value      String
	length     int
	lengthProp valueProperty
}

func newStringValue(s string) String {
	if u := unistring.Scan(s); u != nil {
		return unicodeString(u)
	}
	return asciiString(s)
}

func stringValueFromRaw(raw unistring.String) String {
	if b := raw.AsUtf16(); b != nil {
		return unicodeString(b)
	}
	return asciiString(raw)
}

func (s *stringObject) init() {
	s.baseObject.init()
	s.setLength()
}

func (s *stringObject) setLength() {
	if s.value != nil {
		s.length = s.value.Length()
	}
	s.lengthProp.value = intToValue(int64(s.length))
	s._put("length", &s.lengthProp)
}

func (s *stringObject) getStr(name unistring.String, receiver Value) Value {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		return s._getIdx(i)
	}
	return s.baseObject.getStr(name, receiver)
}

func (s *stringObject) getIdx(idx valueInt, receiver Value) Value {
	i := int(idx)
	if i >= 0 && i < s.length {
		return s._getIdx(i)
	}
	return s.baseObject.getStr(idx.string(), receiver)
}

func (s *stringObject) getOwnPropStr(name unistring.String) Value {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		val := s._getIdx(i)
		return &valueProperty{
			value:      val,
			enumerable: true,
		}
	}

	return s.baseObject.getOwnPropStr(name)
}

func (s *stringObject) getOwnPropIdx(idx valueInt) Value {
	i := int64(idx)
	if i >= 0 {
		if i < int64(s.length) {
			val := s._getIdx(int(i))
			return &valueProperty{
				value:      val,
				enumerable: true,
			}
		}
		return nil
	}

	return s.baseObject.getOwnPropStr(idx.string())
}

func (s *stringObject) _getIdx(idx int) Value {
	return s.value.Substring(idx, idx+1)
}

func (s *stringObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		s.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%d' of a String", i)
		return false
	}

	return s.baseObject.setOwnStr(name, val, throw)
}

func (s *stringObject) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	i := int64(idx)
	if i >= 0 && i < int64(s.length) {
		s.val.runtime.typeErrorResult(throw, "Cannot assign to read only property '%d' of a String", i)
		return false
	}

	return s.baseObject.setOwnStr(idx.string(), val, throw)
}

func (s *stringObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return s._setForeignStr(name, s.getOwnPropStr(name), val, receiver, throw)
}

func (s *stringObject) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return s._setForeignIdx(idx, s.getOwnPropIdx(idx), val, receiver, throw)
}

func (s *stringObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		_, ok := s._defineOwnProperty(name, &valueProperty{enumerable: true}, descr, throw)
		return ok
	}

	return s.baseObject.defineOwnPropertyStr(name, descr, throw)
}

func (s *stringObject) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	i := int64(idx)
	if i >= 0 && i < int64(s.length) {
		s.val.runtime.typeErrorResult(throw, "Cannot redefine property: %d", i)
		return false
	}

	return s.baseObject.defineOwnPropertyStr(idx.string(), descr, throw)
}

type stringPropIter struct {
	str         String // separate, because obj can be the singleton
	obj         *stringObject
	idx, length int
}

func (i *stringPropIter) next() (propIterItem, iterNextFunc) {
	if i.idx < i.length {
		name := strconv.Itoa(i.idx)
		i.idx++
		return propIterItem{name: asciiString(name), enumerable: _ENUM_TRUE}, i.next
	}

	return i.obj.baseObject.iterateStringKeys()()
}

func (s *stringObject) iterateStringKeys() iterNextFunc {
	return (&stringPropIter{
		str:    s.value,
		obj:    s,
		length: s.length,
	}).next
}

func (s *stringObject) stringKeys(all bool, accum []Value) []Value {
	for i := 0; i < s.length; i++ {
		accum = append(accum, asciiString(strconv.Itoa(i)))
	}

	return s.baseObject.stringKeys(all, accum)
}

func (s *stringObject) deleteStr(name unistring.String, throw bool) bool {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		s.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of a String", i)
		return false
	}

	return s.baseObject.deleteStr(name, throw)
}

func (s *stringObject) deleteIdx(idx valueInt, throw bool) bool {
	i := int64(idx)
	if i >= 0 && i < int64(s.length) {
		s.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of a String", i)
		return false
	}

	return s.baseObject.deleteStr(idx.string(), throw)
}

func (s *stringObject) hasOwnPropertyStr(name unistring.String) bool {
	if i := strToGoIdx(name); i >= 0 && i < s.length {
		return true
	}
	return s.baseObject.hasOwnPropertyStr(name)
}

func (s *stringObject) hasOwnPropertyIdx(idx valueInt) bool {
	i := int64(idx)
	if i >= 0 && i < int64(s.length) {
		return true
	}
	return s.baseObject.hasOwnPropertyStr(idx.string())
}

func devirtualizeString(s String) (asciiString, unicodeString) {
	switch s := s.(type) {
	case asciiString:
		return s, nil
	case unicodeString:
		return "", s
	case *importedString:
		s.ensureScanned()
		if s.u != nil {
			return "", s.u
		}
		return asciiString(s.s), nil
	default:
		panic(unknownStringTypeErr(s))
	}
}

func unknownStringTypeErr(v Value) interface{} {
	return newTypeError("Internal bug: unknown string type: %T", v)
}

// StringFromUTF16 creates a string value from an array of UTF-16 code units. The result is a copy, so the initial
// slice can be modified after calling this function (but it must not be modified while the function is running).
// No validation of any kind is performed.
func StringFromUTF16(chars []uint16) String {
	isAscii := true
	for _, c := range chars {
		if c >= utf8.RuneSelf {
			isAscii = false
			break
		}
	}
	if isAscii {
		var sb strings.Builder
		sb.Grow(len(chars))
		for _, c := range chars {
			sb.WriteByte(byte(c))
		}
		return asciiString(sb.String())
	}
	buf := make([]uint16, len(chars)+1)
	buf[0] = unistring.BOM
	copy(buf[1:], chars)
	return unicodeString(buf)
}
