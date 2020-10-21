package goja

import (
	"fmt"
	"github.com/dop251/goja/parser"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func (r *Runtime) newRegexpObject(proto *Object) *regexpObject {
	v := &Object{runtime: r}

	o := &regexpObject{}
	o.class = classRegExp
	o.val = v
	o.extensible = true
	v.self = o
	o.prototype = proto
	o.init()
	return o
}

func (r *Runtime) newRegExpp(pattern *regexpPattern, patternStr valueString, proto *Object) *Object {
	o := r.newRegexpObject(proto)

	o.pattern = pattern
	o.source = patternStr

	return o.val
}

func decodeHex(s string) (int, bool) {
	var hex int
	for i := 0; i < len(s); i++ {
		var n byte
		chr := s[i]
		switch {
		case '0' <= chr && chr <= '9':
			n = chr - '0'
		case 'a' <= chr && chr <= 'f':
			n = chr - 'a' + 10
		case 'A' <= chr && chr <= 'F':
			n = chr - 'A' + 10
		default:
			return 0, false
		}
		hex = hex*16 + int(n)
	}
	return hex, true
}

func writeHex4(b *strings.Builder, i int) {
	b.WriteByte(hex[i>>12])
	b.WriteByte(hex[(i>>8)&0xF])
	b.WriteByte(hex[(i>>4)&0xF])
	b.WriteByte(hex[i&0xF])
}

// Convert any valid surrogate pairs in the form of \uXXXX\uXXXX to unicode characters
func convertRegexpToUnicode(patternStr string) string {
	var sb strings.Builder
	pos := 0
	for i := 0; i < len(patternStr)-11; {
		r, size := utf8.DecodeRuneInString(patternStr[i:])
		if r == '\\' {
			i++
			if patternStr[i] == 'u' && patternStr[i+5] == '\\' && patternStr[i+6] == 'u' {
				if first, ok := decodeHex(patternStr[i+1 : i+5]); ok {
					if isUTF16FirstSurrogate(rune(first)) {
						if second, ok := decodeHex(patternStr[i+7 : i+11]); ok {
							if isUTF16SecondSurrogate(rune(second)) {
								r = utf16.DecodeRune(rune(first), rune(second))
								sb.WriteString(patternStr[pos : i-1])
								sb.WriteRune(r)
								i += 11
								pos = i
								continue
							}
						}
					}
				}
			}
			i++
		} else {
			i += size
		}
	}
	if pos > 0 {
		sb.WriteString(patternStr[pos:])
		return sb.String()
	}
	return patternStr
}

// Convert any extended unicode characters to UTF-16 in the form of \uXXXX\uXXXX
func convertRegexpToUtf16(patternStr string) string {
	var sb strings.Builder
	pos := 0
	var prevRune rune
	for i := 0; i < len(patternStr); {
		r, size := utf8.DecodeRuneInString(patternStr[i:])
		if r > 0xFFFF {
			sb.WriteString(patternStr[pos:i])
			if prevRune == '\\' {
				sb.WriteRune('\\')
			}
			first, second := utf16.EncodeRune(r)
			sb.WriteString(`\u`)
			writeHex4(&sb, int(first))
			sb.WriteString(`\u`)
			writeHex4(&sb, int(second))
			pos = i + size
		}
		i += size
		prevRune = r
	}
	if pos > 0 {
		sb.WriteString(patternStr[pos:])
		return sb.String()
	}
	return patternStr
}

// convert any broken UTF-16 surrogate pairs to \uXXXX
func escapeInvalidUtf16(s valueString) string {
	if ascii, ok := s.(asciiString); ok {
		return ascii.String()
	}
	var sb strings.Builder
	rd := &lenientUtf16Decoder{utf16Reader: s.utf16Reader(0)}
	pos := 0
	utf8Size := 0
	var utf8Buf [utf8.UTFMax]byte
	for {
		c, size, err := rd.ReadRune()
		if err != nil {
			break
		}
		if utf16.IsSurrogate(c) {
			if sb.Len() == 0 {
				sb.Grow(utf8Size + 7)
				hrd := s.reader(0)
				var c rune
				for p := 0; p < pos; {
					var size int
					var err error
					c, size, err = hrd.ReadRune()
					if err != nil {
						// will not happen
						panic(fmt.Errorf("error while reading string head %q, pos: %d: %w", s.String(), pos, err))
					}
					sb.WriteRune(c)
					p += size
				}
				if c == '\\' {
					sb.WriteRune(c)
				}
			}
			sb.WriteString(`\u`)
			writeHex4(&sb, int(c))
		} else {
			if sb.Len() > 0 {
				sb.WriteRune(c)
			} else {
				utf8Size += utf8.EncodeRune(utf8Buf[:], c)
				pos += size
			}
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	return s.String()
}

func compileRegexpFromValueString(patternStr valueString, flags string) (*regexpPattern, error) {
	return compileRegexp(escapeInvalidUtf16(patternStr), flags)
}

func compileRegexp(patternStr, flags string) (p *regexpPattern, err error) {
	var global, ignoreCase, multiline, sticky, unicode bool
	var wrapper *regexpWrapper
	var wrapper2 *regexp2Wrapper

	if flags != "" {
		invalidFlags := func() {
			err = fmt.Errorf("Invalid flags supplied to RegExp constructor '%s'", flags)
		}
		for _, chr := range flags {
			switch chr {
			case 'g':
				if global {
					invalidFlags()
					return
				}
				global = true
			case 'm':
				if multiline {
					invalidFlags()
					return
				}
				multiline = true
			case 'i':
				if ignoreCase {
					invalidFlags()
					return
				}
				ignoreCase = true
			case 'y':
				if sticky {
					invalidFlags()
					return
				}
				sticky = true
			case 'u':
				if unicode {
					invalidFlags()
				}
				unicode = true
			default:
				invalidFlags()
				return
			}
		}
	}

	if unicode {
		patternStr = convertRegexpToUnicode(patternStr)
	} else {
		patternStr = convertRegexpToUtf16(patternStr)
	}

	re2Str, err1 := parser.TransformRegExp(patternStr)
	if err1 == nil {
		re2flags := ""
		if multiline {
			re2flags += "m"
		}
		if ignoreCase {
			re2flags += "i"
		}
		if len(re2flags) > 0 {
			re2Str = fmt.Sprintf("(?%s:%s)", re2flags, re2Str)
		}

		pattern, err1 := regexp.Compile(re2Str)
		if err1 != nil {
			err = fmt.Errorf("Invalid regular expression (re2): %s (%v)", re2Str, err1)
			return
		}
		wrapper = (*regexpWrapper)(pattern)
	} else {
		if re2Str == "" {
			err = err1
			return
		}
		wrapper2, err = compileRegexp2(patternStr, multiline, ignoreCase)
		if err != nil {
			err = fmt.Errorf("Invalid regular expression (regexp2): %s (%v)", patternStr, err)
			return
		}
	}

	p = &regexpPattern{
		src:            patternStr,
		regexpWrapper:  wrapper,
		regexp2Wrapper: wrapper2,
		global:         global,
		ignoreCase:     ignoreCase,
		multiline:      multiline,
		sticky:         sticky,
		unicode:        unicode,
	}
	return
}

func (r *Runtime) _newRegExp(patternStr valueString, flags string, proto *Object) *Object {
	pattern, err := compileRegexpFromValueString(patternStr, flags)
	if err != nil {
		panic(r.newSyntaxError(err.Error(), -1))
	}
	return r.newRegExpp(pattern, patternStr, proto)
}

func (r *Runtime) builtin_newRegExp(args []Value, proto *Object) *Object {
	var patternVal, flagsVal Value
	if len(args) > 0 {
		patternVal = args[0]
	}
	if len(args) > 1 {
		flagsVal = args[1]
	}
	return r.newRegExp(patternVal, flagsVal, proto)
}

func (r *Runtime) newRegExp(patternVal, flagsVal Value, proto *Object) *Object {
	var pattern valueString
	var flags string
	if obj, ok := patternVal.(*Object); ok {
		if rx, ok := obj.self.(*regexpObject); ok {
			if flagsVal == nil || flagsVal == _undefined {
				return rx.clone()
			} else {
				return r._newRegExp(rx.source, flagsVal.toString().String(), proto)
			}
		} else {
			if isRegexp(patternVal) {
				pattern = nilSafe(obj.self.getStr("source", nil)).toString()
				if flagsVal == nil || flagsVal == _undefined {
					flags = nilSafe(obj.self.getStr("flags", nil)).toString().String()
				} else {
					flags = flagsVal.toString().String()
				}
				goto exit
			}
		}
	}

	if patternVal != nil && patternVal != _undefined {
		pattern = patternVal.toString()
	}
	if flagsVal != nil && flagsVal != _undefined {
		flags = flagsVal.toString().String()
	}

	if pattern == nil {
		pattern = stringEmpty
	}
exit:
	return r._newRegExp(pattern, flags, proto)
}

func (r *Runtime) builtin_RegExp(call FunctionCall) Value {
	pattern := call.Argument(0)
	patternIsRegExp := isRegexp(pattern)
	flags := call.Argument(1)
	if patternIsRegExp && flags == _undefined {
		if obj, ok := call.Argument(0).(*Object); ok {
			patternConstructor := obj.self.getStr("constructor", nil)
			if patternConstructor == r.global.RegExp {
				return pattern
			}
		}
	}
	return r.newRegExp(pattern, flags, r.global.RegExpPrototype)
}

func (r *Runtime) regexpproto_compile(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		var (
			pattern *regexpPattern
			source  valueString
			flags   string
			err     error
		)
		patternVal := call.Argument(0)
		flagsVal := call.Argument(1)
		if o, ok := patternVal.(*Object); ok {
			if p, ok := o.self.(*regexpObject); ok {
				if flagsVal != _undefined {
					panic(r.NewTypeError("Cannot supply flags when constructing one RegExp from another"))
				}
				this.pattern = p.pattern
				this.source = p.source
				goto exit
			}
		}
		if patternVal != _undefined {
			source = patternVal.toString()
		} else {
			source = stringEmpty
		}
		if flagsVal != _undefined {
			flags = flagsVal.toString().String()
		}
		pattern, err = compileRegexpFromValueString(source, flags)
		if err != nil {
			panic(r.newSyntaxError(err.Error(), -1))
		}
		this.pattern = pattern
		this.source = source
	exit:
		this.setOwnStr("lastIndex", intToValue(0), true)
		return call.This
	}

	panic(r.NewTypeError("Method RegExp.prototype.compile called on incompatible receiver %s", call.This.toString()))
}

func (r *Runtime) regexpproto_exec(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		return this.exec(call.Argument(0).toString())
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.exec called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_test(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.test(call.Argument(0).toString()) {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.test called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_toString(call FunctionCall) Value {
	obj := r.toObject(call.This)
	if this := r.checkStdRegexp(obj); this != nil {
		var sb valueStringBuilder
		sb.WriteRune('/')
		if !this.writeEscapedSource(&sb) {
			sb.WriteString(this.source)
		}
		sb.WriteRune('/')
		if this.pattern.global {
			sb.WriteRune('g')
		}
		if this.pattern.ignoreCase {
			sb.WriteRune('i')
		}
		if this.pattern.multiline {
			sb.WriteRune('m')
		}
		if this.pattern.unicode {
			sb.WriteRune('u')
		}
		if this.pattern.sticky {
			sb.WriteRune('y')
		}
		return sb.String()
	}
	pattern := nilSafe(obj.self.getStr("source", nil)).toString()
	flags := nilSafe(obj.self.getStr("flags", nil)).toString()
	var sb valueStringBuilder
	sb.WriteRune('/')
	sb.WriteString(pattern)
	sb.WriteRune('/')
	sb.WriteString(flags)
	return sb.String()
}

func (r *regexpObject) writeEscapedSource(sb *valueStringBuilder) bool {
	if r.source.length() == 0 {
		sb.WriteString(asciiString("(?:)"))
		return true
	}
	pos := 0
	lastPos := 0
	rd := &lenientUtf16Decoder{utf16Reader: r.source.utf16Reader(0)}
L:
	for {
		c, size, err := rd.ReadRune()
		if err != nil {
			break
		}
		switch c {
		case '\\':
			pos++
			_, size, err = rd.ReadRune()
			if err != nil {
				break L
			}
		case '/', '\u000a', '\u000d', '\u2028', '\u2029':
			sb.WriteSubstring(r.source, lastPos, pos)
			sb.WriteRune('\\')
			switch c {
			case '\u000a':
				sb.WriteRune('n')
			case '\u000d':
				sb.WriteRune('r')
			default:
				sb.WriteRune('u')
				sb.WriteRune(rune(hex[c>>12]))
				sb.WriteRune(rune(hex[(c>>8)&0xF]))
				sb.WriteRune(rune(hex[(c>>4)&0xF]))
				sb.WriteRune(rune(hex[c&0xF]))
			}
			lastPos = pos + size
		}
		pos += size
	}
	if lastPos > 0 {
		sb.WriteSubstring(r.source, lastPos, r.source.length())
		return true
	}
	return false
}

func (r *Runtime) regexpproto_getSource(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		var sb valueStringBuilder
		if this.writeEscapedSource(&sb) {
			return sb.String()
		}
		return this.source
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.source getter called on incompatible receiver")
		return nil
	}
}

func (r *Runtime) regexpproto_getGlobal(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.pattern.global {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.global getter called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_getMultiline(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.pattern.multiline {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.multiline getter called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_getIgnoreCase(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.pattern.ignoreCase {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.ignoreCase getter called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_getUnicode(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.pattern.unicode {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.unicode getter called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_getSticky(call FunctionCall) Value {
	if this, ok := r.toObject(call.This).self.(*regexpObject); ok {
		if this.pattern.sticky {
			return valueTrue
		} else {
			return valueFalse
		}
	} else {
		r.typeErrorResult(true, "Method RegExp.prototype.sticky getter called on incompatible receiver %s", call.This.toString())
		return nil
	}
}

func (r *Runtime) regexpproto_getFlags(call FunctionCall) Value {
	var global, ignoreCase, multiline, sticky, unicode bool

	thisObj := r.toObject(call.This)
	size := 0
	if v := thisObj.self.getStr("global", nil); v != nil {
		global = v.ToBoolean()
		if global {
			size++
		}
	}
	if v := thisObj.self.getStr("ignoreCase", nil); v != nil {
		ignoreCase = v.ToBoolean()
		if ignoreCase {
			size++
		}
	}
	if v := thisObj.self.getStr("multiline", nil); v != nil {
		multiline = v.ToBoolean()
		if multiline {
			size++
		}
	}
	if v := thisObj.self.getStr("sticky", nil); v != nil {
		sticky = v.ToBoolean()
		if sticky {
			size++
		}
	}
	if v := thisObj.self.getStr("unicode", nil); v != nil {
		unicode = v.ToBoolean()
		if unicode {
			size++
		}
	}

	var sb strings.Builder
	sb.Grow(size)
	if global {
		sb.WriteByte('g')
	}
	if ignoreCase {
		sb.WriteByte('i')
	}
	if multiline {
		sb.WriteByte('m')
	}
	if unicode {
		sb.WriteByte('u')
	}
	if sticky {
		sb.WriteByte('y')
	}

	return asciiString(sb.String())
}

func (r *Runtime) regExpExec(execFn func(FunctionCall) Value, rxObj *Object, arg Value) Value {
	res := execFn(FunctionCall{
		This:      rxObj,
		Arguments: []Value{arg},
	})

	if res != _null {
		if _, ok := res.(*Object); !ok {
			panic(r.NewTypeError("RegExp exec method returned something other than an Object or null"))
		}
	}

	return res
}

func (r *Runtime) getGlobalRegexpMatches(rxObj *Object, s valueString) []Value {
	fullUnicode := nilSafe(rxObj.self.getStr("unicode", nil)).ToBoolean()
	rxObj.self.setOwnStr("lastIndex", intToValue(0), true)
	execFn, ok := r.toObject(rxObj.self.getStr("exec", nil)).self.assertCallable()
	if !ok {
		panic(r.NewTypeError("exec is not a function"))
	}
	var a []Value
	for {
		res := r.regExpExec(execFn, rxObj, s)
		if res == _null {
			break
		}
		a = append(a, res)
		matchStr := nilSafe(r.toObject(res).self.getIdx(valueInt(0), nil)).toString()
		if matchStr.length() == 0 {
			thisIndex := toIntStrict(nilSafe(rxObj.self.getStr("lastIndex", nil)).ToInteger())
			rxObj.self.setOwnStr("lastIndex", valueInt(advanceStringIndex(s, thisIndex, fullUnicode)), true)
		}
	}

	return a
}

func (r *Runtime) regexpproto_stdMatcherGeneric(rxObj *Object, s valueString) Value {
	rx := rxObj.self
	global := rx.getStr("global", nil)
	if global != nil && global.ToBoolean() {
		a := r.getGlobalRegexpMatches(rxObj, s)
		if len(a) == 0 {
			return _null
		}
		ar := make([]Value, 0, len(a))
		for _, result := range a {
			obj := r.toObject(result)
			matchStr := nilSafe(obj.self.getIdx(valueInt(0), nil)).ToString()
			ar = append(ar, matchStr)
		}
		return r.newArrayValues(ar)
	}

	execFn, ok := r.toObject(rx.getStr("exec", nil)).self.assertCallable()
	if !ok {
		panic(r.NewTypeError("exec is not a function"))
	}

	return r.regExpExec(execFn, rxObj, s)
}

func (r *Runtime) checkStdRegexp(rxObj *Object) *regexpObject {
	if deoptimiseRegexp {
		return nil
	}

	rx, ok := rxObj.self.(*regexpObject)
	if !ok {
		return nil
	}

	if !rx.standard || rx.prototype == nil || rx.prototype.self != r.global.stdRegexpProto {
		return nil
	}

	return rx
}

func (r *Runtime) regexpproto_stdMatcher(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	s := call.Argument(0).toString()
	rx := r.checkStdRegexp(thisObj)
	if rx == nil {
		return r.regexpproto_stdMatcherGeneric(thisObj, s)
	}
	if rx.pattern.global {
		res := rx.pattern.findAllSubmatchIndex(s, 0, -1, rx.pattern.sticky)
		if len(res) == 0 {
			rx.setOwnStr("lastIndex", intToValue(0), true)
			return _null
		}
		a := make([]Value, 0, len(res))
		for _, result := range res {
			a = append(a, s.substring(result[0], result[1]))
		}
		rx.setOwnStr("lastIndex", intToValue(int64(res[len(res)-1][1])), true)
		return r.newArrayValues(a)
	} else {
		return rx.exec(s)
	}
}

func (r *Runtime) regexpproto_stdSearchGeneric(rxObj *Object, arg valueString) Value {
	rx := rxObj.self
	previousLastIndex := nilSafe(rx.getStr("lastIndex", nil))
	zero := intToValue(0)
	if !previousLastIndex.SameAs(zero) {
		rx.setOwnStr("lastIndex", zero, true)
	}
	execFn, ok := r.toObject(rx.getStr("exec", nil)).self.assertCallable()
	if !ok {
		panic(r.NewTypeError("exec is not a function"))
	}

	result := r.regExpExec(execFn, rxObj, arg)
	currentLastIndex := nilSafe(rx.getStr("lastIndex", nil))
	if !currentLastIndex.SameAs(previousLastIndex) {
		rx.setOwnStr("lastIndex", previousLastIndex, true)
	}

	if result == _null {
		return intToValue(-1)
	}

	return r.toObject(result).self.getStr("index", nil)
}

func (r *Runtime) regexpproto_stdSearch(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	s := call.Argument(0).toString()
	rx := r.checkStdRegexp(thisObj)
	if rx == nil {
		return r.regexpproto_stdSearchGeneric(thisObj, s)
	}

	previousLastIndex := rx.getStr("lastIndex", nil)
	rx.setOwnStr("lastIndex", intToValue(0), true)

	match, result := rx.execRegexp(s)
	rx.setOwnStr("lastIndex", previousLastIndex, true)

	if !match {
		return intToValue(-1)
	}
	return intToValue(int64(result[0]))
}

func (r *Runtime) regexpproto_stdSplitterGeneric(splitter *Object, s valueString, limit Value, unicodeMatching bool) Value {
	var a []Value
	var lim int64
	if limit == nil || limit == _undefined {
		lim = maxInt - 1
	} else {
		lim = toLength(limit)
	}
	if lim == 0 {
		return r.newArrayValues(a)
	}
	size := s.length()
	p := 0
	execFn := toMethod(splitter.ToObject(r).self.getStr("exec", nil)) // must be non-nil

	if size == 0 {
		if r.regExpExec(execFn, splitter, s) == _null {
			a = append(a, s)
		}
		return r.newArrayValues(a)
	}

	q := p
	for q < size {
		splitter.self.setOwnStr("lastIndex", intToValue(int64(q)), true)
		z := r.regExpExec(execFn, splitter, s)
		if z == _null {
			q = advanceStringIndex(s, q, unicodeMatching)
		} else {
			z := r.toObject(z)
			e := toLength(splitter.self.getStr("lastIndex", nil))
			if e == int64(p) {
				q = advanceStringIndex(s, q, unicodeMatching)
			} else {
				a = append(a, s.substring(p, q))
				if int64(len(a)) == lim {
					return r.newArrayValues(a)
				}
				if e > int64(size) {
					p = size
				} else {
					p = int(e)
				}
				numberOfCaptures := max(toLength(z.self.getStr("length", nil))-1, 0)
				for i := int64(1); i <= numberOfCaptures; i++ {
					a = append(a, z.self.getIdx(valueInt(i), nil))
					if int64(len(a)) == lim {
						return r.newArrayValues(a)
					}
				}
				q = p
			}
		}
	}
	a = append(a, s.substring(p, size))
	return r.newArrayValues(a)
}

func advanceStringIndex(s valueString, pos int, unicode bool) int {
	next := pos + 1
	if !unicode {
		return next
	}
	l := s.length()
	if next >= l {
		return next
	}
	if !isUTF16FirstSurrogate(s.charAt(pos)) {
		return next
	}
	if !isUTF16SecondSurrogate(s.charAt(next)) {
		return next
	}
	return next + 1
}

func (r *Runtime) regexpproto_stdSplitter(call FunctionCall) Value {
	rxObj := r.toObject(call.This)
	s := call.Argument(0).toString()
	limitValue := call.Argument(1)
	var splitter *Object
	search := r.checkStdRegexp(rxObj)
	c := r.speciesConstructorObj(rxObj, r.global.RegExp)
	if search == nil || c != r.global.RegExp {
		flags := nilSafe(rxObj.self.getStr("flags", nil)).toString()
		flagsStr := flags.String()

		// Add 'y' flag if missing
		if !strings.Contains(flagsStr, "y") {
			flags = flags.concat(asciiString("y"))
		}
		splitter = r.toConstructor(c)([]Value{rxObj, flags}, nil)
		search = r.checkStdRegexp(splitter)
		if search == nil {
			return r.regexpproto_stdSplitterGeneric(splitter, s, limitValue, strings.Contains(flagsStr, "u"))
		}
	}

	limit := -1
	if limitValue != _undefined {
		limit = int(toUint32(limitValue))
	}

	if limit == 0 {
		return r.newArrayValues(nil)
	}

	targetLength := s.length()
	var valueArray []Value
	lastIndex := 0
	found := 0

	result := search.pattern.findAllSubmatchIndex(s, 0, -1, false)
	if targetLength == 0 {
		if result == nil {
			valueArray = append(valueArray, s)
		}
		goto RETURN
	}

	for _, match := range result {
		if match[0] == match[1] {
			// FIXME Ugh, this is a hack
			if match[0] == 0 || match[0] == targetLength {
				continue
			}
		}

		if lastIndex != match[0] {
			valueArray = append(valueArray, s.substring(lastIndex, match[0]))
			found++
		} else if lastIndex == match[0] {
			if lastIndex != -1 {
				valueArray = append(valueArray, stringEmpty)
				found++
			}
		}

		lastIndex = match[1]
		if found == limit {
			goto RETURN
		}

		captureCount := len(match) / 2
		for index := 1; index < captureCount; index++ {
			offset := index * 2
			var value Value
			if match[offset] != -1 {
				value = s.substring(match[offset], match[offset+1])
			} else {
				value = _undefined
			}
			valueArray = append(valueArray, value)
			found++
			if found == limit {
				goto RETURN
			}
		}
	}

	if found != limit {
		if lastIndex != targetLength {
			valueArray = append(valueArray, s.substring(lastIndex, targetLength))
		} else {
			valueArray = append(valueArray, stringEmpty)
		}
	}

RETURN:
	return r.newArrayValues(valueArray)
}

func (r *Runtime) regexpproto_stdReplacerGeneric(rxObj *Object, s, replaceStr valueString, rcall func(FunctionCall) Value) Value {
	var results []Value
	if nilSafe(rxObj.self.getStr("global", nil)).ToBoolean() {
		results = r.getGlobalRegexpMatches(rxObj, s)
	} else {
		execFn := toMethod(rxObj.self.getStr("exec", nil)) // must be non-nil
		result := r.regExpExec(execFn, rxObj, s)
		if result != _null {
			results = append(results, result)
		}
	}
	lengthS := s.length()
	nextSourcePosition := 0
	var resultBuf valueStringBuilder
	for _, result := range results {
		obj := r.toObject(result)
		nCaptures := max(toLength(obj.self.getStr("length", nil))-1, 0)
		matched := nilSafe(obj.self.getIdx(valueInt(0), nil)).toString()
		matchLength := matched.length()
		position := toIntStrict(max(min(nilSafe(obj.self.getStr("index", nil)).ToInteger(), int64(lengthS)), 0))
		var captures []Value
		if rcall != nil {
			captures = make([]Value, 0, nCaptures+3)
		} else {
			captures = make([]Value, 0, nCaptures+1)
		}
		captures = append(captures, matched)
		for n := int64(1); n <= nCaptures; n++ {
			capN := nilSafe(obj.self.getIdx(valueInt(n), nil))
			if capN != _undefined {
				capN = capN.ToString()
			}
			captures = append(captures, capN)
		}
		var replacement valueString
		if rcall != nil {
			captures = append(captures, intToValue(int64(position)), s)
			replacement = rcall(FunctionCall{
				This:      _undefined,
				Arguments: captures,
			}).toString()
			if position >= nextSourcePosition {
				resultBuf.WriteString(s.substring(nextSourcePosition, position))
				resultBuf.WriteString(replacement)
				nextSourcePosition = position + matchLength
			}
		} else {
			if position >= nextSourcePosition {
				resultBuf.WriteString(s.substring(nextSourcePosition, position))
				writeSubstitution(s, position, len(captures), func(idx int) valueString {
					capture := captures[idx]
					if capture != _undefined {
						return capture.toString()
					}
					return stringEmpty
				}, replaceStr, &resultBuf)
				nextSourcePosition = position + matchLength
			}
		}
	}
	if nextSourcePosition < lengthS {
		resultBuf.WriteString(s.substring(nextSourcePosition, lengthS))
	}
	return resultBuf.String()
}

func writeSubstitution(s valueString, position int, numCaptures int, getCapture func(int) valueString, replaceStr valueString, buf *valueStringBuilder) {
	l := s.length()
	rl := replaceStr.length()
	matched := getCapture(0)
	tailPos := position + matched.length()

	for i := 0; i < rl; i++ {
		c := replaceStr.charAt(i)
		if c == '$' && i < rl-1 {
			ch := replaceStr.charAt(i + 1)
			switch ch {
			case '$':
				buf.WriteRune('$')
			case '`':
				buf.WriteString(s.substring(0, position))
			case '\'':
				if tailPos < l {
					buf.WriteString(s.substring(tailPos, l))
				}
			case '&':
				buf.WriteString(matched)
			default:
				matchNumber := 0
				j := i + 1
				for j < rl {
					ch := replaceStr.charAt(j)
					if ch >= '0' && ch <= '9' {
						m := matchNumber*10 + int(ch-'0')
						if m >= numCaptures {
							break
						}
						matchNumber = m
						j++
					} else {
						break
					}
				}
				if matchNumber > 0 {
					buf.WriteString(getCapture(matchNumber))
					i = j - 1
					continue
				} else {
					buf.WriteRune('$')
					buf.WriteRune(ch)
				}
			}
			i++
		} else {
			buf.WriteRune(c)
		}
	}
}

func (r *Runtime) regexpproto_stdReplacer(call FunctionCall) Value {
	rxObj := r.toObject(call.This)
	s := call.Argument(0).toString()
	replaceStr, rcall := getReplaceValue(call.Argument(1))

	rx := r.checkStdRegexp(rxObj)
	if rx == nil {
		return r.regexpproto_stdReplacerGeneric(rxObj, s, replaceStr, rcall)
	}

	var index int64
	find := 1
	if rx.pattern.global {
		find = -1
		rx.setOwnStr("lastIndex", intToValue(0), true)
	} else {
		index = rx.getLastIndex()
	}
	found := rx.pattern.findAllSubmatchIndex(s, toIntStrict(index), find, rx.pattern.sticky)
	if len(found) > 0 {
		if !rx.updateLastIndex(index, found[0], found[len(found)-1]) {
			found = nil
		}
	} else {
		rx.updateLastIndex(index, nil, nil)
	}

	return stringReplace(s, found, replaceStr, rcall)
}

func (r *Runtime) initRegExp() {
	o := r.newGuardedObject(r.global.ObjectPrototype, classObject)
	r.global.RegExpPrototype = o.val
	r.global.stdRegexpProto = o
	o._putProp("compile", r.newNativeFunc(r.regexpproto_compile, nil, "compile", nil, 2), true, false, true)
	o._putProp("exec", r.newNativeFunc(r.regexpproto_exec, nil, "exec", nil, 1), true, false, true)
	o._putProp("test", r.newNativeFunc(r.regexpproto_test, nil, "test", nil, 1), true, false, true)
	o._putProp("toString", r.newNativeFunc(r.regexpproto_toString, nil, "toString", nil, 0), true, false, true)
	o.setOwnStr("source", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getSource, nil, "get source", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("global", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getGlobal, nil, "get global", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("multiline", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getMultiline, nil, "get multiline", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("ignoreCase", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getIgnoreCase, nil, "get ignoreCase", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("unicode", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getUnicode, nil, "get unicode", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("sticky", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getSticky, nil, "get sticky", nil, 0),
		accessor:     true,
	}, false)
	o.setOwnStr("flags", &valueProperty{
		configurable: true,
		getterFunc:   r.newNativeFunc(r.regexpproto_getFlags, nil, "get flags", nil, 0),
		accessor:     true,
	}, false)

	o._putSym(symMatch, valueProp(r.newNativeFunc(r.regexpproto_stdMatcher, nil, "[Symbol.match]", nil, 1), true, false, true))
	o._putSym(symSearch, valueProp(r.newNativeFunc(r.regexpproto_stdSearch, nil, "[Symbol.search]", nil, 1), true, false, true))
	o._putSym(symSplit, valueProp(r.newNativeFunc(r.regexpproto_stdSplitter, nil, "[Symbol.split]", nil, 2), true, false, true))
	o._putSym(symReplace, valueProp(r.newNativeFunc(r.regexpproto_stdReplacer, nil, "[Symbol.replace]", nil, 2), true, false, true))
	o.guard("exec", "global", "multiline", "ignoreCase", "unicode", "sticky")

	r.global.RegExp = r.newNativeFunc(r.builtin_RegExp, r.builtin_newRegExp, "RegExp", r.global.RegExpPrototype, 2)
	rx := r.global.RegExp.self
	rx._putSym(symSpecies, &valueProperty{
		getterFunc:   r.newNativeFunc(r.returnThis, nil, "get [Symbol.species]", nil, 0),
		accessor:     true,
		configurable: true,
	})
	r.addToGlobal("RegExp", r.global.RegExp)
}
