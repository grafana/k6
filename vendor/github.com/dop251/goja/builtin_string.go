package goja

import (
	"github.com/dop251/goja/unistring"
	"math"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dop251/goja/parser"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func (r *Runtime) collator() *collate.Collator {
	collator := r._collator
	if collator == nil {
		collator = collate.New(language.Und)
		r._collator = collator
	}
	return collator
}

func toString(arg Value) valueString {
	if s, ok := arg.(valueString); ok {
		return s
	}
	if s, ok := arg.(*Symbol); ok {
		return s.descriptiveString()
	}
	return arg.toString()
}

func (r *Runtime) builtin_String(call FunctionCall) Value {
	if len(call.Arguments) > 0 {
		return toString(call.Arguments[0])
	} else {
		return stringEmpty
	}
}

func (r *Runtime) _newString(s valueString, proto *Object) *Object {
	v := &Object{runtime: r}

	o := &stringObject{}
	o.class = classString
	o.val = v
	o.extensible = true
	v.self = o
	o.prototype = proto
	if s != nil {
		o.value = s
	}
	o.init()
	return v
}

func (r *Runtime) builtin_newString(args []Value, proto *Object) *Object {
	var s valueString
	if len(args) > 0 {
		s = args[0].toString()
	} else {
		s = stringEmpty
	}
	return r._newString(s, proto)
}

func (r *Runtime) stringproto_toStringValueOf(this Value, funcName string) Value {
	if str, ok := this.(valueString); ok {
		return str
	}
	if obj, ok := this.(*Object); ok {
		if strObj, ok := obj.self.(*stringObject); ok {
			return strObj.value
		}
	}
	r.typeErrorResult(true, "String.prototype.%s is called on incompatible receiver", funcName)
	return nil
}

func (r *Runtime) stringproto_toString(call FunctionCall) Value {
	return r.stringproto_toStringValueOf(call.This, "toString")
}

func (r *Runtime) stringproto_valueOf(call FunctionCall) Value {
	return r.stringproto_toStringValueOf(call.This, "valueOf")
}

func (r *Runtime) stringproto_iterator(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	return r.createStringIterator(call.This.toString())
}

func (r *Runtime) string_fromcharcode(call FunctionCall) Value {
	b := make([]byte, len(call.Arguments))
	for i, arg := range call.Arguments {
		chr := toUint16(arg)
		if chr >= utf8.RuneSelf {
			bb := make([]uint16, len(call.Arguments)+1)
			bb[0] = unistring.BOM
			bb1 := bb[1:]
			for j := 0; j < i; j++ {
				bb1[j] = uint16(b[j])
			}
			bb1[i] = chr
			i++
			for j, arg := range call.Arguments[i:] {
				bb1[i+j] = toUint16(arg)
			}
			return unicodeString(bb)
		}
		b[i] = byte(chr)
	}

	return asciiString(b)
}

func (r *Runtime) string_fromcodepoint(call FunctionCall) Value {
	var sb valueStringBuilder
	for _, arg := range call.Arguments {
		num := arg.ToNumber()
		var c rune
		if numInt, ok := num.(valueInt); ok {
			if numInt < 0 || numInt > utf8.MaxRune {
				panic(r.newError(r.global.RangeError, "Invalid code point %d", numInt))
			}
			c = rune(numInt)
		} else {
			panic(r.newError(r.global.RangeError, "Invalid code point %s", num))
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

func (r *Runtime) string_raw(call FunctionCall) Value {
	cooked := call.Argument(0).ToObject(r)
	raw := nilSafe(cooked.self.getStr("raw", nil)).ToObject(r)
	literalSegments := toLength(raw.self.getStr("length", nil))
	if literalSegments <= 0 {
		return stringEmpty
	}
	var stringElements valueStringBuilder
	nextIndex := int64(0)
	numberOfSubstitutions := int64(len(call.Arguments) - 1)
	for {
		nextSeg := nilSafe(raw.self.getIdx(valueInt(nextIndex), nil)).toString()
		stringElements.WriteString(nextSeg)
		if nextIndex+1 == literalSegments {
			return stringElements.String()
		}
		if nextIndex < numberOfSubstitutions {
			stringElements.WriteString(nilSafe(call.Arguments[nextIndex+1]).toString())
		}
		nextIndex++
	}
}

func (r *Runtime) stringproto_charAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	pos := call.Argument(0).ToInteger()
	if pos < 0 || pos >= int64(s.length()) {
		return stringEmpty
	}
	return newStringValue(string(s.charAt(toIntStrict(pos))))
}

func (r *Runtime) stringproto_charCodeAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	pos := call.Argument(0).ToInteger()
	if pos < 0 || pos >= int64(s.length()) {
		return _NaN
	}
	return intToValue(int64(s.charAt(toIntStrict(pos)) & 0xFFFF))
}

func (r *Runtime) stringproto_codePointAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	p := call.Argument(0).ToInteger()
	size := s.length()
	if p < 0 || p >= int64(size) {
		return _undefined
	}
	pos := toIntStrict(p)
	first := s.charAt(pos)
	if isUTF16FirstSurrogate(first) {
		pos++
		if pos < size {
			second := s.charAt(pos)
			if isUTF16SecondSurrogate(second) {
				return intToValue(int64(utf16.DecodeRune(first, second)))
			}
		}
	}
	return intToValue(int64(first & 0xFFFF))
}

func (r *Runtime) stringproto_concat(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	strs := make([]valueString, len(call.Arguments)+1)
	strs[0] = call.This.toString()
	_, allAscii := strs[0].(asciiString)
	totalLen := strs[0].length()
	for i, arg := range call.Arguments {
		s := arg.toString()
		if allAscii {
			_, allAscii = s.(asciiString)
		}
		strs[i+1] = s
		totalLen += s.length()
	}

	if allAscii {
		var buf strings.Builder
		buf.Grow(totalLen)
		for _, s := range strs {
			buf.WriteString(s.String())
		}
		return asciiString(buf.String())
	} else {
		buf := make([]uint16, totalLen+1)
		buf[0] = unistring.BOM
		pos := 1
		for _, s := range strs {
			switch s := s.(type) {
			case asciiString:
				for i := 0; i < len(s); i++ {
					buf[pos] = uint16(s[i])
					pos++
				}
			case unicodeString:
				copy(buf[pos:], s[1:])
				pos += s.length()
			}
		}
		return unicodeString(buf)
	}
}

func (r *Runtime) stringproto_endsWith(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	searchString := call.Argument(0)
	if isRegexp(searchString) {
		panic(r.NewTypeError("First argument to String.prototype.endsWith must not be a regular expression"))
	}
	searchStr := searchString.toString()
	l := int64(s.length())
	var pos int64
	if posArg := call.Argument(1); posArg != _undefined {
		pos = posArg.ToInteger()
	} else {
		pos = l
	}
	end := toIntStrict(min(max(pos, 0), l))
	searchLength := searchStr.length()
	start := end - searchLength
	if start < 0 {
		return valueFalse
	}
	for i := 0; i < searchLength; i++ {
		if s.charAt(start+i) != searchStr.charAt(i) {
			return valueFalse
		}
	}
	return valueTrue
}

func (r *Runtime) stringproto_includes(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	searchString := call.Argument(0)
	if isRegexp(searchString) {
		panic(r.NewTypeError("First argument to String.prototype.includes must not be a regular expression"))
	}
	searchStr := searchString.toString()
	var pos int64
	if posArg := call.Argument(1); posArg != _undefined {
		pos = posArg.ToInteger()
	} else {
		pos = 0
	}
	start := toIntStrict(min(max(pos, 0), int64(s.length())))
	if s.index(searchStr, start) != -1 {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) stringproto_indexOf(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	value := call.This.toString()
	target := call.Argument(0).toString()
	pos := call.Argument(1).ToInteger()

	if pos < 0 {
		pos = 0
	} else {
		l := int64(value.length())
		if pos > l {
			pos = l
		}
	}

	return intToValue(int64(value.index(target, toIntStrict(pos))))
}

func (r *Runtime) stringproto_lastIndexOf(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	value := call.This.toString()
	target := call.Argument(0).toString()
	numPos := call.Argument(1).ToNumber()

	var pos int64
	if f, ok := numPos.(valueFloat); ok && math.IsNaN(float64(f)) {
		pos = int64(value.length())
	} else {
		pos = numPos.ToInteger()
		if pos < 0 {
			pos = 0
		} else {
			l := int64(value.length())
			if pos > l {
				pos = l
			}
		}
	}

	return intToValue(int64(value.lastIndex(target, toIntStrict(pos))))
}

func (r *Runtime) stringproto_localeCompare(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	this := norm.NFD.String(call.This.toString().String())
	that := norm.NFD.String(call.Argument(0).toString().String())
	return intToValue(int64(r.collator().CompareString(this, that)))
}

func (r *Runtime) stringproto_match(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	regexp := call.Argument(0)
	if regexp != _undefined && regexp != _null {
		if matcher := toMethod(r.getV(regexp, SymMatch)); matcher != nil {
			return matcher(FunctionCall{
				This:      regexp,
				Arguments: []Value{call.This},
			})
		}
	}

	var rx *regexpObject
	if regexp, ok := regexp.(*Object); ok {
		rx, _ = regexp.self.(*regexpObject)
	}

	if rx == nil {
		rx = r.newRegExp(regexp, nil, r.global.RegExpPrototype)
	}

	if matcher, ok := r.toObject(rx.getSym(SymMatch, nil)).self.assertCallable(); ok {
		return matcher(FunctionCall{
			This:      rx.val,
			Arguments: []Value{call.This.toString()},
		})
	}

	panic(r.NewTypeError("RegExp matcher is not a function"))
}

func (r *Runtime) stringproto_matchAll(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	regexp := call.Argument(0)
	if regexp != _undefined && regexp != _null {
		if isRegexp(regexp) {
			if o, ok := regexp.(*Object); ok {
				flags := o.Get("flags")
				r.checkObjectCoercible(flags)
				if !strings.Contains(flags.toString().String(), "g") {
					panic(r.NewTypeError("RegExp doesn't have global flag set"))
				}
			}
		}
		if matcher := toMethod(r.getV(regexp, SymMatchAll)); matcher != nil {
			return matcher(FunctionCall{
				This:      regexp,
				Arguments: []Value{call.This},
			})
		}
	}

	rx := r.newRegExp(regexp, asciiString("g"), r.global.RegExpPrototype)

	if matcher, ok := r.toObject(rx.getSym(SymMatchAll, nil)).self.assertCallable(); ok {
		return matcher(FunctionCall{
			This:      rx.val,
			Arguments: []Value{call.This.toString()},
		})
	}

	panic(r.NewTypeError("RegExp matcher is not a function"))
}

func (r *Runtime) stringproto_normalize(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	var form string
	if formArg := call.Argument(0); formArg != _undefined {
		form = formArg.toString().toString().String()
	} else {
		form = "NFC"
	}
	var f norm.Form
	switch form {
	case "NFC":
		f = norm.NFC
	case "NFD":
		f = norm.NFD
	case "NFKC":
		f = norm.NFKC
	case "NFKD":
		f = norm.NFKD
	default:
		panic(r.newError(r.global.RangeError, "The normalization form should be one of NFC, NFD, NFKC, NFKD"))
	}

	if s, ok := s.(unicodeString); ok {
		ss := s.String()
		return newStringValue(f.String(ss))
	}

	return s
}

func (r *Runtime) stringproto_padEnd(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	maxLength := toLength(call.Argument(0))
	stringLength := int64(s.length())
	if maxLength <= stringLength {
		return s
	}
	var filler valueString
	var fillerASCII bool
	if fillString := call.Argument(1); fillString != _undefined {
		filler = fillString.toString()
		if filler.length() == 0 {
			return s
		}
		_, fillerASCII = filler.(asciiString)
	} else {
		filler = asciiString(" ")
		fillerASCII = true
	}
	remaining := toIntStrict(maxLength - stringLength)
	_, stringASCII := s.(asciiString)
	if fillerASCII && stringASCII {
		fl := filler.length()
		var sb strings.Builder
		sb.Grow(toIntStrict(maxLength))
		sb.WriteString(s.String())
		fs := filler.String()
		for remaining >= fl {
			sb.WriteString(fs)
			remaining -= fl
		}
		if remaining > 0 {
			sb.WriteString(fs[:remaining])
		}
		return asciiString(sb.String())
	}
	var sb unicodeStringBuilder
	sb.Grow(toIntStrict(maxLength))
	sb.WriteString(s)
	fl := filler.length()
	for remaining >= fl {
		sb.WriteString(filler)
		remaining -= fl
	}
	if remaining > 0 {
		sb.WriteString(filler.substring(0, remaining))
	}

	return sb.String()
}

func (r *Runtime) stringproto_padStart(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	maxLength := toLength(call.Argument(0))
	stringLength := int64(s.length())
	if maxLength <= stringLength {
		return s
	}
	var filler valueString
	var fillerASCII bool
	if fillString := call.Argument(1); fillString != _undefined {
		filler = fillString.toString()
		if filler.length() == 0 {
			return s
		}
		_, fillerASCII = filler.(asciiString)
	} else {
		filler = asciiString(" ")
		fillerASCII = true
	}
	remaining := toIntStrict(maxLength - stringLength)
	_, stringASCII := s.(asciiString)
	if fillerASCII && stringASCII {
		fl := filler.length()
		var sb strings.Builder
		sb.Grow(toIntStrict(maxLength))
		fs := filler.String()
		for remaining >= fl {
			sb.WriteString(fs)
			remaining -= fl
		}
		if remaining > 0 {
			sb.WriteString(fs[:remaining])
		}
		sb.WriteString(s.String())
		return asciiString(sb.String())
	}
	var sb unicodeStringBuilder
	sb.Grow(toIntStrict(maxLength))
	fl := filler.length()
	for remaining >= fl {
		sb.WriteString(filler)
		remaining -= fl
	}
	if remaining > 0 {
		sb.WriteString(filler.substring(0, remaining))
	}
	sb.WriteString(s)

	return sb.String()
}

func (r *Runtime) stringproto_repeat(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	n := call.Argument(0).ToNumber()
	if n == _positiveInf {
		panic(r.newError(r.global.RangeError, "Invalid count value"))
	}
	numInt := n.ToInteger()
	if numInt < 0 {
		panic(r.newError(r.global.RangeError, "Invalid count value"))
	}
	if numInt == 0 || s.length() == 0 {
		return stringEmpty
	}
	num := toIntStrict(numInt)
	if s, ok := s.(asciiString); ok {
		var sb strings.Builder
		sb.Grow(len(s) * num)
		for i := 0; i < num; i++ {
			sb.WriteString(string(s))
		}
		return asciiString(sb.String())
	}

	var sb unicodeStringBuilder
	sb.Grow(s.length() * num)
	for i := 0; i < num; i++ {
		sb.WriteString(s)
	}
	return sb.String()
}

func getReplaceValue(replaceValue Value) (str valueString, rcall func(FunctionCall) Value) {
	if replaceValue, ok := replaceValue.(*Object); ok {
		if c, ok := replaceValue.self.assertCallable(); ok {
			rcall = c
			return
		}
	}
	str = replaceValue.toString()
	return
}

func stringReplace(s valueString, found [][]int, newstring valueString, rcall func(FunctionCall) Value) Value {
	if len(found) == 0 {
		return s
	}

	var str string
	var isASCII bool
	if astr, ok := s.(asciiString); ok {
		str = string(astr)
		isASCII = true
	}

	var buf valueStringBuilder

	lastIndex := 0
	lengthS := s.length()
	if rcall != nil {
		for _, item := range found {
			if item[0] != lastIndex {
				buf.WriteSubstring(s, lastIndex, item[0])
			}
			matchCount := len(item) / 2
			argumentList := make([]Value, matchCount+2)
			for index := 0; index < matchCount; index++ {
				offset := 2 * index
				if item[offset] != -1 {
					if isASCII {
						argumentList[index] = asciiString(str[item[offset]:item[offset+1]])
					} else {
						argumentList[index] = s.substring(item[offset], item[offset+1])
					}
				} else {
					argumentList[index] = _undefined
				}
			}
			argumentList[matchCount] = valueInt(item[0])
			argumentList[matchCount+1] = s
			replacement := rcall(FunctionCall{
				This:      _undefined,
				Arguments: argumentList,
			}).toString()
			buf.WriteString(replacement)
			lastIndex = item[1]
		}
	} else {
		for _, item := range found {
			if item[0] != lastIndex {
				buf.WriteString(s.substring(lastIndex, item[0]))
			}
			matchCount := len(item) / 2
			writeSubstitution(s, item[0], matchCount, func(idx int) valueString {
				if item[idx*2] != -1 {
					if isASCII {
						return asciiString(str[item[idx*2]:item[idx*2+1]])
					}
					return s.substring(item[idx*2], item[idx*2+1])
				}
				return stringEmpty
			}, newstring, &buf)
			lastIndex = item[1]
		}
	}

	if lastIndex != lengthS {
		buf.WriteString(s.substring(lastIndex, lengthS))
	}

	return buf.String()
}

func (r *Runtime) stringproto_replace(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	searchValue := call.Argument(0)
	replaceValue := call.Argument(1)
	if searchValue != _undefined && searchValue != _null {
		if replacer := toMethod(r.getV(searchValue, SymReplace)); replacer != nil {
			return replacer(FunctionCall{
				This:      searchValue,
				Arguments: []Value{call.This, replaceValue},
			})
		}
	}

	s := call.This.toString()
	var found [][]int
	searchStr := searchValue.toString()
	pos := s.index(searchStr, 0)
	if pos != -1 {
		found = append(found, []int{pos, pos + searchStr.length()})
	}

	str, rcall := getReplaceValue(replaceValue)
	return stringReplace(s, found, str, rcall)
}

func (r *Runtime) stringproto_search(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	regexp := call.Argument(0)
	if regexp != _undefined && regexp != _null {
		if searcher := toMethod(r.getV(regexp, SymSearch)); searcher != nil {
			return searcher(FunctionCall{
				This:      regexp,
				Arguments: []Value{call.This},
			})
		}
	}

	var rx *regexpObject
	if regexp, ok := regexp.(*Object); ok {
		rx, _ = regexp.self.(*regexpObject)
	}

	if rx == nil {
		rx = r.newRegExp(regexp, nil, r.global.RegExpPrototype)
	}

	if searcher, ok := r.toObject(rx.getSym(SymSearch, nil)).self.assertCallable(); ok {
		return searcher(FunctionCall{
			This:      rx.val,
			Arguments: []Value{call.This.toString()},
		})
	}

	panic(r.NewTypeError("RegExp searcher is not a function"))
}

func (r *Runtime) stringproto_slice(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	l := int64(s.length())
	start := call.Argument(0).ToInteger()
	var end int64
	if arg1 := call.Argument(1); arg1 != _undefined {
		end = arg1.ToInteger()
	} else {
		end = l
	}

	if start < 0 {
		start += l
		if start < 0 {
			start = 0
		}
	} else {
		if start > l {
			start = l
		}
	}

	if end < 0 {
		end += l
		if end < 0 {
			end = 0
		}
	} else {
		if end > l {
			end = l
		}
	}

	if end > start {
		return s.substring(int(start), int(end))
	}
	return stringEmpty
}

func (r *Runtime) stringproto_split(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	separatorValue := call.Argument(0)
	limitValue := call.Argument(1)
	if separatorValue != _undefined && separatorValue != _null {
		if splitter := toMethod(r.getV(separatorValue, SymSplit)); splitter != nil {
			return splitter(FunctionCall{
				This:      separatorValue,
				Arguments: []Value{call.This, limitValue},
			})
		}
	}
	s := call.This.toString()

	limit := -1
	if limitValue != _undefined {
		limit = int(toUint32(limitValue))
	}

	separatorValue = separatorValue.ToString()

	if limit == 0 {
		return r.newArrayValues(nil)
	}

	if separatorValue == _undefined {
		return r.newArrayValues([]Value{s})
	}

	separator := separatorValue.String()

	excess := false
	str := s.String()
	if limit > len(str) {
		limit = len(str)
	}
	splitLimit := limit
	if limit > 0 {
		splitLimit = limit + 1
		excess = true
	}

	// TODO handle invalid UTF-16
	split := strings.SplitN(str, separator, splitLimit)

	if excess && len(split) > limit {
		split = split[:limit]
	}

	valueArray := make([]Value, len(split))
	for index, value := range split {
		valueArray[index] = newStringValue(value)
	}

	return r.newArrayValues(valueArray)
}

func (r *Runtime) stringproto_startsWith(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	searchString := call.Argument(0)
	if isRegexp(searchString) {
		panic(r.NewTypeError("First argument to String.prototype.startsWith must not be a regular expression"))
	}
	searchStr := searchString.toString()
	l := int64(s.length())
	var pos int64
	if posArg := call.Argument(1); posArg != _undefined {
		pos = posArg.ToInteger()
	}
	start := toIntStrict(min(max(pos, 0), l))
	searchLength := searchStr.length()
	if int64(searchLength+start) > l {
		return valueFalse
	}
	for i := 0; i < searchLength; i++ {
		if s.charAt(start+i) != searchStr.charAt(i) {
			return valueFalse
		}
	}
	return valueTrue
}

func (r *Runtime) stringproto_substring(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	l := int64(s.length())
	intStart := call.Argument(0).ToInteger()
	var intEnd int64
	if end := call.Argument(1); end != _undefined {
		intEnd = end.ToInteger()
	} else {
		intEnd = l
	}
	if intStart < 0 {
		intStart = 0
	} else if intStart > l {
		intStart = l
	}

	if intEnd < 0 {
		intEnd = 0
	} else if intEnd > l {
		intEnd = l
	}

	if intStart > intEnd {
		intStart, intEnd = intEnd, intStart
	}

	return s.substring(int(intStart), int(intEnd))
}

func (r *Runtime) stringproto_toLowerCase(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	return s.toLower()
}

func (r *Runtime) stringproto_toUpperCase(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	return s.toUpper()
}

func (r *Runtime) stringproto_trim(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	// TODO handle invalid UTF-16
	return newStringValue(strings.Trim(s.String(), parser.WhitespaceChars))
}

func (r *Runtime) stringproto_trimEnd(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	// TODO handle invalid UTF-16
	return newStringValue(strings.TrimRight(s.String(), parser.WhitespaceChars))
}

func (r *Runtime) stringproto_trimStart(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	// TODO handle invalid UTF-16
	return newStringValue(strings.TrimLeft(s.String(), parser.WhitespaceChars))
}

func (r *Runtime) stringproto_substr(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	start := call.Argument(0).ToInteger()
	var length int64
	sl := int64(s.length())
	if arg := call.Argument(1); arg != _undefined {
		length = arg.ToInteger()
	} else {
		length = sl
	}

	if start < 0 {
		start = max(sl+start, 0)
	}

	length = min(max(length, 0), sl-start)
	if length <= 0 {
		return stringEmpty
	}

	return s.substring(int(start), int(start+length))
}

func (r *Runtime) stringIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*stringIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method String Iterator.prototype.next called on incompatible receiver %s", thisObj.String()))
}

func (r *Runtime) createStringIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.IteratorPrototype, classObject)

	o._putProp("next", r.newNativeFunc(r.stringIterProto_next, nil, "next", nil, 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classStringIterator), false, false, true))

	return o
}

func (r *Runtime) initString() {
	r.global.StringIteratorPrototype = r.newLazyObject(r.createStringIterProto)
	r.global.StringPrototype = r.builtin_newString([]Value{stringEmpty}, r.global.ObjectPrototype)

	o := r.global.StringPrototype.self
	o._putProp("charAt", r.newNativeFunc(r.stringproto_charAt, nil, "charAt", nil, 1), true, false, true)
	o._putProp("charCodeAt", r.newNativeFunc(r.stringproto_charCodeAt, nil, "charCodeAt", nil, 1), true, false, true)
	o._putProp("codePointAt", r.newNativeFunc(r.stringproto_codePointAt, nil, "codePointAt", nil, 1), true, false, true)
	o._putProp("concat", r.newNativeFunc(r.stringproto_concat, nil, "concat", nil, 1), true, false, true)
	o._putProp("endsWith", r.newNativeFunc(r.stringproto_endsWith, nil, "endsWith", nil, 1), true, false, true)
	o._putProp("includes", r.newNativeFunc(r.stringproto_includes, nil, "includes", nil, 1), true, false, true)
	o._putProp("indexOf", r.newNativeFunc(r.stringproto_indexOf, nil, "indexOf", nil, 1), true, false, true)
	o._putProp("lastIndexOf", r.newNativeFunc(r.stringproto_lastIndexOf, nil, "lastIndexOf", nil, 1), true, false, true)
	o._putProp("localeCompare", r.newNativeFunc(r.stringproto_localeCompare, nil, "localeCompare", nil, 1), true, false, true)
	o._putProp("match", r.newNativeFunc(r.stringproto_match, nil, "match", nil, 1), true, false, true)
	o._putProp("matchAll", r.newNativeFunc(r.stringproto_matchAll, nil, "matchAll", nil, 1), true, false, true)
	o._putProp("normalize", r.newNativeFunc(r.stringproto_normalize, nil, "normalize", nil, 0), true, false, true)
	o._putProp("padEnd", r.newNativeFunc(r.stringproto_padEnd, nil, "padEnd", nil, 1), true, false, true)
	o._putProp("padStart", r.newNativeFunc(r.stringproto_padStart, nil, "padStart", nil, 1), true, false, true)
	o._putProp("repeat", r.newNativeFunc(r.stringproto_repeat, nil, "repeat", nil, 1), true, false, true)
	o._putProp("replace", r.newNativeFunc(r.stringproto_replace, nil, "replace", nil, 2), true, false, true)
	o._putProp("search", r.newNativeFunc(r.stringproto_search, nil, "search", nil, 1), true, false, true)
	o._putProp("slice", r.newNativeFunc(r.stringproto_slice, nil, "slice", nil, 2), true, false, true)
	o._putProp("split", r.newNativeFunc(r.stringproto_split, nil, "split", nil, 2), true, false, true)
	o._putProp("startsWith", r.newNativeFunc(r.stringproto_startsWith, nil, "startsWith", nil, 1), true, false, true)
	o._putProp("substring", r.newNativeFunc(r.stringproto_substring, nil, "substring", nil, 2), true, false, true)
	o._putProp("toLocaleLowerCase", r.newNativeFunc(r.stringproto_toLowerCase, nil, "toLocaleLowerCase", nil, 0), true, false, true)
	o._putProp("toLocaleUpperCase", r.newNativeFunc(r.stringproto_toUpperCase, nil, "toLocaleUpperCase", nil, 0), true, false, true)
	o._putProp("toLowerCase", r.newNativeFunc(r.stringproto_toLowerCase, nil, "toLowerCase", nil, 0), true, false, true)
	o._putProp("toString", r.newNativeFunc(r.stringproto_toString, nil, "toString", nil, 0), true, false, true)
	o._putProp("toUpperCase", r.newNativeFunc(r.stringproto_toUpperCase, nil, "toUpperCase", nil, 0), true, false, true)
	o._putProp("trim", r.newNativeFunc(r.stringproto_trim, nil, "trim", nil, 0), true, false, true)
	trimEnd := r.newNativeFunc(r.stringproto_trimEnd, nil, "trimEnd", nil, 0)
	trimStart := r.newNativeFunc(r.stringproto_trimStart, nil, "trimStart", nil, 0)
	o._putProp("trimEnd", trimEnd, true, false, true)
	o._putProp("trimStart", trimStart, true, false, true)
	o._putProp("trimRight", trimEnd, true, false, true)
	o._putProp("trimLeft", trimStart, true, false, true)
	o._putProp("valueOf", r.newNativeFunc(r.stringproto_valueOf, nil, "valueOf", nil, 0), true, false, true)

	o._putSym(SymIterator, valueProp(r.newNativeFunc(r.stringproto_iterator, nil, "[Symbol.iterator]", nil, 0), true, false, true))

	// Annex B
	o._putProp("substr", r.newNativeFunc(r.stringproto_substr, nil, "substr", nil, 2), true, false, true)

	r.global.String = r.newNativeFunc(r.builtin_String, r.builtin_newString, "String", r.global.StringPrototype, 1)
	o = r.global.String.self
	o._putProp("fromCharCode", r.newNativeFunc(r.string_fromcharcode, nil, "fromCharCode", nil, 1), true, false, true)
	o._putProp("fromCodePoint", r.newNativeFunc(r.string_fromcodepoint, nil, "fromCodePoint", nil, 1), true, false, true)
	o._putProp("raw", r.newNativeFunc(r.string_raw, nil, "raw", nil, 1), true, false, true)

	r.addToGlobal("String", r.global.String)

	r.stringSingleton = r.builtin_new(r.global.String, nil).self.(*stringObject)
}
