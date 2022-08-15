package goja

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja/unistring"
)

const (
	__proto__ = "__proto__"
)

var (
	stringTrue        valueString = asciiString("true")
	stringFalse       valueString = asciiString("false")
	stringNull        valueString = asciiString("null")
	stringUndefined   valueString = asciiString("undefined")
	stringObjectC     valueString = asciiString("object")
	stringFunction    valueString = asciiString("function")
	stringBoolean     valueString = asciiString("boolean")
	stringString      valueString = asciiString("string")
	stringSymbol      valueString = asciiString("symbol")
	stringNumber      valueString = asciiString("number")
	stringNaN         valueString = asciiString("NaN")
	stringInfinity                = asciiString("Infinity")
	stringNegInfinity             = asciiString("-Infinity")
	stringBound_      valueString = asciiString("bound ")
	stringEmpty       valueString = asciiString("")

	stringError          valueString = asciiString("Error")
	stringAggregateError valueString = asciiString("AggregateError")
	stringTypeError      valueString = asciiString("TypeError")
	stringReferenceError valueString = asciiString("ReferenceError")
	stringSyntaxError    valueString = asciiString("SyntaxError")
	stringRangeError     valueString = asciiString("RangeError")
	stringEvalError      valueString = asciiString("EvalError")
	stringURIError       valueString = asciiString("URIError")
	stringGoError        valueString = asciiString("GoError")

	stringObjectNull      valueString = asciiString("[object Null]")
	stringObjectUndefined valueString = asciiString("[object Undefined]")
	stringInvalidDate     valueString = asciiString("Invalid Date")
)

type valueString interface {
	Value
	charAt(int) rune
	length() int
	concat(valueString) valueString
	substring(start, end int) valueString
	compareTo(valueString) int
	reader() io.RuneReader
	utf16Reader() io.RuneReader
	utf16Runes() []rune
	index(valueString, int) int
	lastIndex(valueString, int) int
	toLower() valueString
	toUpper() valueString
	toTrimmedUTF8() string
}

type stringIterObject struct {
	baseObject
	reader io.RuneReader
}

func isUTF16FirstSurrogate(r rune) bool {
	return r >= 0xD800 && r <= 0xDBFF
}

func isUTF16SecondSurrogate(r rune) bool {
	return r >= 0xDC00 && r <= 0xDFFF
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

func stringFromRune(r rune) valueString {
	if r < utf8.RuneSelf {
		var sb strings.Builder
		sb.WriteByte(byte(r))
		return asciiString(sb.String())
	}
	var sb unicodeStringBuilder
	sb.WriteRune(r)
	return sb.String()
}

func (r *Runtime) createStringIterator(s valueString) Value {
	o := &Object{runtime: r}

	si := &stringIterObject{
		reader: &lenientUtf16Decoder{utf16Reader: s.utf16Reader()},
	}
	si.class = classStringIterator
	si.val = o
	si.extensible = true
	o.self = si
	si.prototype = r.global.StringIteratorPrototype
	si.init()

	return o
}

type stringObject struct {
	baseObject
	value      valueString
	length     int
	lengthProp valueProperty
}

func newStringValue(s string) valueString {
	if u := unistring.Scan(s); u != nil {
		return unicodeString(u)
	}
	return asciiString(s)
}

func stringValueFromRaw(raw unistring.String) valueString {
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
		s.length = s.value.length()
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
	return s.value.substring(idx, idx+1)
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
	str         valueString // separate, because obj can be the singleton
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

func devirtualizeString(s valueString) (asciiString, unicodeString) {
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
