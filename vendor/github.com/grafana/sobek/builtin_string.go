package sobek

import (
	"math"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/grafana/sobek/parser"
	"github.com/grafana/sobek/unistring"

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

func toString(arg Value) String {
	if s, ok := arg.(String); ok {
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

func (r *Runtime) _newString(s String, proto *Object) *Object {
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
	var s String
	if len(args) > 0 {
		s = args[0].toString()
	} else {
		s = stringEmpty
	}
	return r._newString(s, proto)
}

func (r *Runtime) stringproto_toStringValueOf(this Value, funcName string) Value {
	if str, ok := this.(String); ok {
		return str
	}
	if obj, ok := this.(*Object); ok {
		if strObj, ok := obj.self.(*stringObject); ok {
			return strObj.value
		}
		if reflectObj, ok := obj.self.(*objectGoReflect); ok && reflectObj.class == classString {
			if toString := reflectObj.toString; toString != nil {
				return toString()
			}
			if valueOf := reflectObj.valueOf; valueOf != nil {
				return valueOf()
			}
		}
		if obj == r.global.StringPrototype {
			return stringEmpty
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
	var sb StringBuilder
	for _, arg := range call.Arguments {
		num := arg.ToNumber()
		var c rune
		if numInt, ok := num.(valueInt); ok {
			if numInt < 0 || numInt > utf8.MaxRune {
				panic(r.newError(r.getRangeError(), "Invalid code point %d", numInt))
			}
			c = rune(numInt)
		} else {
			panic(r.newError(r.getRangeError(), "Invalid code point %s", num))
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
	var stringElements StringBuilder
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

func (r *Runtime) stringproto_at(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	pos := call.Argument(0).ToInteger()
	length := int64(s.Length())
	if pos < 0 {
		pos = length + pos
	}
	if pos >= length || pos < 0 {
		return _undefined
	}
	return s.Substring(int(pos), int(pos+1))
}

func (r *Runtime) stringproto_charAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	pos := call.Argument(0).ToInteger()
	if pos < 0 || pos >= int64(s.Length()) {
		return stringEmpty
	}
	return s.Substring(int(pos), int(pos+1))
}

func (r *Runtime) stringproto_charCodeAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	pos := call.Argument(0).ToInteger()
	if pos < 0 || pos >= int64(s.Length()) {
		return _NaN
	}
	return intToValue(int64(s.CharAt(toIntStrict(pos)) & 0xFFFF))
}

func (r *Runtime) stringproto_codePointAt(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	p := call.Argument(0).ToInteger()
	size := s.Length()
	if p < 0 || p >= int64(size) {
		return _undefined
	}
	pos := toIntStrict(p)
	first := s.CharAt(pos)
	if isUTF16FirstSurrogate(first) {
		pos++
		if pos < size {
			second := s.CharAt(pos)
			if isUTF16SecondSurrogate(second) {
				return intToValue(int64(utf16.DecodeRune(rune(first), rune(second))))
			}
		}
	}
	return intToValue(int64(first & 0xFFFF))
}

func (r *Runtime) stringproto_concat(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	strs := make([]String, len(call.Arguments)+1)
	a, u := devirtualizeString(call.This.toString())
	allAscii := true
	totalLen := 0
	if u == nil {
		strs[0] = a
		totalLen = len(a)
	} else {
		strs[0] = u
		totalLen = u.Length()
		allAscii = false
	}
	for i, arg := range call.Arguments {
		a, u := devirtualizeString(arg.toString())
		if u != nil {
			allAscii = false
			totalLen += u.Length()
			strs[i+1] = u
		} else {
			totalLen += a.Length()
			strs[i+1] = a
		}
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
				pos += s.Length()
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
	l := int64(s.Length())
	var pos int64
	if posArg := call.Argument(1); posArg != _undefined {
		pos = posArg.ToInteger()
	} else {
		pos = l
	}
	end := toIntStrict(min(max(pos, 0), l))
	searchLength := searchStr.Length()
	start := end - searchLength
	if start < 0 {
		return valueFalse
	}
	for i := 0; i < searchLength; i++ {
		if s.CharAt(start+i) != searchStr.CharAt(i) {
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
	start := toIntStrict(min(max(pos, 0), int64(s.Length())))
	if s.index(searchStr, start) != -1 {
		return valueTrue
	}
	return valueFalse
}

func (r *Runtime) stringproto_indexOf(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	value := call.This.toString()
	target := call.Argument(0).toString()
	pos := call.Argument(1).ToNumber().ToInteger()

	if pos < 0 {
		pos = 0
	} else {
		l := int64(value.Length())
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
		pos = int64(value.Length())
	} else {
		pos = numPos.ToInteger()
		if pos < 0 {
			pos = 0
		} else {
			l := int64(value.Length())
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
		rx = r.newRegExp(regexp, nil, r.getRegExpPrototype())
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
				flags := nilSafe(o.self.getStr("flags", nil))
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

	rx := r.newRegExp(regexp, asciiString("g"), r.getRegExpPrototype())

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
		panic(r.newError(r.getRangeError(), "The normalization form should be one of NFC, NFD, NFKC, NFKD"))
	}

	switch s := s.(type) {
	case asciiString:
		return s
	case unicodeString:
		ss := s.String()
		return newStringValue(f.String(ss))
	case *importedString:
		if s.scanned && s.u == nil {
			return asciiString(s.s)
		}
		return newStringValue(f.String(s.s))
	default:
		panic(unknownStringTypeErr(s))
	}
}

func (r *Runtime) _stringPad(call FunctionCall, start bool) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	maxLength := toLength(call.Argument(0))
	stringLength := int64(s.Length())
	if maxLength <= stringLength {
		return s
	}
	strAscii, strUnicode := devirtualizeString(s)
	var filler String
	var fillerAscii asciiString
	var fillerUnicode unicodeString
	if fillString := call.Argument(1); fillString != _undefined {
		filler = fillString.toString()
		if filler.Length() == 0 {
			return s
		}
		fillerAscii, fillerUnicode = devirtualizeString(filler)
	} else {
		fillerAscii = " "
		filler = fillerAscii
	}
	remaining := toIntStrict(maxLength - stringLength)
	if fillerUnicode == nil && strUnicode == nil {
		fl := fillerAscii.Length()
		var sb strings.Builder
		sb.Grow(toIntStrict(maxLength))
		if !start {
			sb.WriteString(string(strAscii))
		}
		for remaining >= fl {
			sb.WriteString(string(fillerAscii))
			remaining -= fl
		}
		if remaining > 0 {
			sb.WriteString(string(fillerAscii[:remaining]))
		}
		if start {
			sb.WriteString(string(strAscii))
		}
		return asciiString(sb.String())
	}
	var sb unicodeStringBuilder
	sb.ensureStarted(toIntStrict(maxLength))
	if !start {
		sb.writeString(s)
	}
	fl := filler.Length()
	for remaining >= fl {
		sb.writeString(filler)
		remaining -= fl
	}
	if remaining > 0 {
		sb.writeString(filler.Substring(0, remaining))
	}
	if start {
		sb.writeString(s)
	}

	return sb.String()
}

func (r *Runtime) stringproto_padEnd(call FunctionCall) Value {
	return r._stringPad(call, false)
}

func (r *Runtime) stringproto_padStart(call FunctionCall) Value {
	return r._stringPad(call, true)
}

func (r *Runtime) stringproto_repeat(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()
	n := call.Argument(0).ToNumber()
	if n == _positiveInf {
		panic(r.newError(r.getRangeError(), "Invalid count value"))
	}
	numInt := n.ToInteger()
	if numInt < 0 {
		panic(r.newError(r.getRangeError(), "Invalid count value"))
	}
	if numInt == 0 || s.Length() == 0 {
		return stringEmpty
	}
	num := toIntStrict(numInt)
	a, u := devirtualizeString(s)
	if u == nil {
		var sb strings.Builder
		sb.Grow(len(a) * num)
		for i := 0; i < num; i++ {
			sb.WriteString(string(a))
		}
		return asciiString(sb.String())
	}

	var sb unicodeStringBuilder
	sb.Grow(u.Length() * num)
	for i := 0; i < num; i++ {
		sb.writeUnicodeString(u)
	}
	return sb.String()
}

func getReplaceValue(replaceValue Value) (str String, rcall func(FunctionCall) Value) {
	if replaceValue, ok := replaceValue.(*Object); ok {
		if c, ok := replaceValue.self.assertCallable(); ok {
			rcall = c
			return
		}
	}
	str = replaceValue.toString()
	return
}

func stringReplace(s String, found [][]int, newstring String, rcall func(FunctionCall) Value) Value {
	if len(found) == 0 {
		return s
	}

	a, u := devirtualizeString(s)

	var buf StringBuilder

	lastIndex := 0
	lengthS := s.Length()
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
					if u == nil {
						argumentList[index] = a[item[offset]:item[offset+1]]
					} else {
						argumentList[index] = u.Substring(item[offset], item[offset+1])
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
				buf.WriteString(s.Substring(lastIndex, item[0]))
			}
			matchCount := len(item) / 2
			writeSubstitution(s, item[0], matchCount, func(idx int) String {
				if item[idx*2] != -1 {
					if u == nil {
						return a[item[idx*2]:item[idx*2+1]]
					}
					return u.Substring(item[idx*2], item[idx*2+1])
				}
				return stringEmpty
			}, newstring, &buf)
			lastIndex = item[1]
		}
	}

	if lastIndex != lengthS {
		buf.WriteString(s.Substring(lastIndex, lengthS))
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
		found = append(found, []int{pos, pos + searchStr.Length()})
	}

	str, rcall := getReplaceValue(replaceValue)
	return stringReplace(s, found, str, rcall)
}

func (r *Runtime) stringproto_replaceAll(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	searchValue := call.Argument(0)
	replaceValue := call.Argument(1)
	if searchValue != _undefined && searchValue != _null {
		if isRegexp(searchValue) {
			if o, ok := searchValue.(*Object); ok {
				flags := nilSafe(o.self.getStr("flags", nil))
				r.checkObjectCoercible(flags)
				if !strings.Contains(flags.toString().String(), "g") {
					panic(r.NewTypeError("String.prototype.replaceAll called with a non-global RegExp argument"))
				}
			}
		}
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
	searchLength := searchStr.Length()
	advanceBy := toIntStrict(max(1, int64(searchLength)))

	pos := s.index(searchStr, 0)
	for pos != -1 {
		found = append(found, []int{pos, pos + searchLength})
		pos = s.index(searchStr, pos+advanceBy)
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
		rx = r.newRegExp(regexp, nil, r.getRegExpPrototype())
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

	l := int64(s.Length())
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
		return s.Substring(int(start), int(end))
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

	str := s.String()
	splitLimit := limit
	if limit > 0 {
		splitLimit = limit + 1
	}

	// TODO handle invalid UTF-16
	split := strings.SplitN(str, separator, splitLimit)

	if limit > 0 && len(split) > limit {
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
	l := int64(s.Length())
	var pos int64
	if posArg := call.Argument(1); posArg != _undefined {
		pos = posArg.ToInteger()
	}
	start := toIntStrict(min(max(pos, 0), l))
	searchLength := searchStr.Length()
	if int64(searchLength+start) > l {
		return valueFalse
	}
	for i := 0; i < searchLength; i++ {
		if s.CharAt(start+i) != searchStr.CharAt(i) {
			return valueFalse
		}
	}
	return valueTrue
}

func (r *Runtime) stringproto_substring(call FunctionCall) Value {
	r.checkObjectCoercible(call.This)
	s := call.This.toString()

	l := int64(s.Length())
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

	return s.Substring(int(intStart), int(intEnd))
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
	sl := int64(s.Length())
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

	return s.Substring(int(start), int(start+length))
}

func (r *Runtime) stringIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*stringIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method String Iterator.prototype.next called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
}

func (r *Runtime) createStringIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getIteratorPrototype(), classObject)

	o._putProp("next", r.newNativeFunc(r.stringIterProto_next, "next", 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classStringIterator), false, false, true))

	return o
}

func (r *Runtime) getStringIteratorPrototype() *Object {
	var o *Object
	if o = r.global.StringIteratorPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.StringIteratorPrototype = o
		o.self = r.createStringIterProto(o)
	}
	return o
}

func createStringProtoTemplate() *objectTemplate {
	t := newObjectTemplate()
	t.protoFactory = func(r *Runtime) *Object {
		return r.global.ObjectPrototype
	}

	t.putStr("length", func(r *Runtime) Value { return valueProp(intToValue(0), false, false, false) })

	t.putStr("constructor", func(r *Runtime) Value { return valueProp(r.getString(), true, false, true) })

	t.putStr("at", func(r *Runtime) Value { return r.methodProp(r.stringproto_at, "at", 1) })
	t.putStr("charAt", func(r *Runtime) Value { return r.methodProp(r.stringproto_charAt, "charAt", 1) })
	t.putStr("charCodeAt", func(r *Runtime) Value { return r.methodProp(r.stringproto_charCodeAt, "charCodeAt", 1) })
	t.putStr("codePointAt", func(r *Runtime) Value { return r.methodProp(r.stringproto_codePointAt, "codePointAt", 1) })
	t.putStr("concat", func(r *Runtime) Value { return r.methodProp(r.stringproto_concat, "concat", 1) })
	t.putStr("endsWith", func(r *Runtime) Value { return r.methodProp(r.stringproto_endsWith, "endsWith", 1) })
	t.putStr("includes", func(r *Runtime) Value { return r.methodProp(r.stringproto_includes, "includes", 1) })
	t.putStr("indexOf", func(r *Runtime) Value { return r.methodProp(r.stringproto_indexOf, "indexOf", 1) })
	t.putStr("lastIndexOf", func(r *Runtime) Value { return r.methodProp(r.stringproto_lastIndexOf, "lastIndexOf", 1) })
	t.putStr("localeCompare", func(r *Runtime) Value { return r.methodProp(r.stringproto_localeCompare, "localeCompare", 1) })
	t.putStr("match", func(r *Runtime) Value { return r.methodProp(r.stringproto_match, "match", 1) })
	t.putStr("matchAll", func(r *Runtime) Value { return r.methodProp(r.stringproto_matchAll, "matchAll", 1) })
	t.putStr("normalize", func(r *Runtime) Value { return r.methodProp(r.stringproto_normalize, "normalize", 0) })
	t.putStr("padEnd", func(r *Runtime) Value { return r.methodProp(r.stringproto_padEnd, "padEnd", 1) })
	t.putStr("padStart", func(r *Runtime) Value { return r.methodProp(r.stringproto_padStart, "padStart", 1) })
	t.putStr("repeat", func(r *Runtime) Value { return r.methodProp(r.stringproto_repeat, "repeat", 1) })
	t.putStr("replace", func(r *Runtime) Value { return r.methodProp(r.stringproto_replace, "replace", 2) })
	t.putStr("replaceAll", func(r *Runtime) Value { return r.methodProp(r.stringproto_replaceAll, "replaceAll", 2) })
	t.putStr("search", func(r *Runtime) Value { return r.methodProp(r.stringproto_search, "search", 1) })
	t.putStr("slice", func(r *Runtime) Value { return r.methodProp(r.stringproto_slice, "slice", 2) })
	t.putStr("split", func(r *Runtime) Value { return r.methodProp(r.stringproto_split, "split", 2) })
	t.putStr("startsWith", func(r *Runtime) Value { return r.methodProp(r.stringproto_startsWith, "startsWith", 1) })
	t.putStr("substring", func(r *Runtime) Value { return r.methodProp(r.stringproto_substring, "substring", 2) })
	t.putStr("toLocaleLowerCase", func(r *Runtime) Value { return r.methodProp(r.stringproto_toLowerCase, "toLocaleLowerCase", 0) })
	t.putStr("toLocaleUpperCase", func(r *Runtime) Value { return r.methodProp(r.stringproto_toUpperCase, "toLocaleUpperCase", 0) })
	t.putStr("toLowerCase", func(r *Runtime) Value { return r.methodProp(r.stringproto_toLowerCase, "toLowerCase", 0) })
	t.putStr("toString", func(r *Runtime) Value { return r.methodProp(r.stringproto_toString, "toString", 0) })
	t.putStr("toUpperCase", func(r *Runtime) Value { return r.methodProp(r.stringproto_toUpperCase, "toUpperCase", 0) })
	t.putStr("trim", func(r *Runtime) Value { return r.methodProp(r.stringproto_trim, "trim", 0) })
	t.putStr("trimEnd", func(r *Runtime) Value { return valueProp(r.getStringproto_trimEnd(), true, false, true) })
	t.putStr("trimStart", func(r *Runtime) Value { return valueProp(r.getStringproto_trimStart(), true, false, true) })
	t.putStr("trimRight", func(r *Runtime) Value { return valueProp(r.getStringproto_trimEnd(), true, false, true) })
	t.putStr("trimLeft", func(r *Runtime) Value { return valueProp(r.getStringproto_trimStart(), true, false, true) })
	t.putStr("valueOf", func(r *Runtime) Value { return r.methodProp(r.stringproto_valueOf, "valueOf", 0) })

	// Annex B
	t.putStr("substr", func(r *Runtime) Value { return r.methodProp(r.stringproto_substr, "substr", 2) })

	t.putSym(SymIterator, func(r *Runtime) Value {
		return valueProp(r.newNativeFunc(r.stringproto_iterator, "[Symbol.iterator]", 0), true, false, true)
	})

	return t
}

func (r *Runtime) getStringproto_trimEnd() *Object {
	ret := r.global.stringproto_trimEnd
	if ret == nil {
		ret = r.newNativeFunc(r.stringproto_trimEnd, "trimEnd", 0)
		r.global.stringproto_trimEnd = ret
	}
	return ret
}

func (r *Runtime) getStringproto_trimStart() *Object {
	ret := r.global.stringproto_trimStart
	if ret == nil {
		ret = r.newNativeFunc(r.stringproto_trimStart, "trimStart", 0)
		r.global.stringproto_trimStart = ret
	}
	return ret
}

func (r *Runtime) getStringSingleton() *stringObject {
	ret := r.stringSingleton
	if ret == nil {
		ret = r.builtin_new(r.getString(), nil).self.(*stringObject)
		r.stringSingleton = ret
	}
	return ret
}

func (r *Runtime) getString() *Object {
	ret := r.global.String
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.String = ret
		proto := r.getStringPrototype()
		o := r.newNativeFuncAndConstruct(ret, r.builtin_String, r.wrapNativeConstruct(r.builtin_newString, ret, proto), proto, "String", intToValue(1))
		ret.self = o
		o._putProp("fromCharCode", r.newNativeFunc(r.string_fromcharcode, "fromCharCode", 1), true, false, true)
		o._putProp("fromCodePoint", r.newNativeFunc(r.string_fromcodepoint, "fromCodePoint", 1), true, false, true)
		o._putProp("raw", r.newNativeFunc(r.string_raw, "raw", 1), true, false, true)
	}
	return ret
}

var (
	stringProtoTemplate     *objectTemplate
	stringProtoTemplateOnce sync.Once
)

func getStringProtoTemplate() *objectTemplate {
	stringProtoTemplateOnce.Do(func() {
		stringProtoTemplate = createStringProtoTemplate()
	})
	return stringProtoTemplate
}

func (r *Runtime) getStringPrototype() *Object {
	ret := r.global.StringPrototype
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.StringPrototype = ret
		o := r.newTemplatedObject(getStringProtoTemplate(), ret)
		o.class = classString
	}
	return ret
}
