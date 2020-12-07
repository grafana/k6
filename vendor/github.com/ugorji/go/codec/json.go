// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// By default, this json support uses base64 encoding for bytes, because you cannot
// store and read any arbitrary string in json (only unicode).
// However, the user can configre how to encode/decode bytes.
//
// This library specifically supports UTF-8 for encoding and decoding only.
//
// Note that the library will happily encode/decode things which are not valid
// json e.g. a map[int64]string. We do it for consistency. With valid json,
// we will encode and decode appropriately.
// Users can specify their map type if necessary to force it.
//
// We cannot use strconv.(Q|Unq)uote because json quotes/unquotes differently.

import (
	"bytes"
	"encoding/base64"
	"math"
	"strconv"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

//--------------------------------

var jsonLiterals = [...]byte{
	'"', 't', 'r', 'u', 'e', '"',
	'"', 'f', 'a', 'l', 's', 'e', '"',
	'"', 'n', 'u', 'l', 'l', '"',
}

const (
	jsonLitTrueQ  = 0
	jsonLitTrue   = 1
	jsonLitFalseQ = 6
	jsonLitFalse  = 7
	jsonLitNullQ  = 13
	jsonLitNull   = 14
)

var (
	// jsonLiteralTrueQ  = jsonLiterals[jsonLitTrueQ : jsonLitTrueQ+6]
	// jsonLiteralFalseQ = jsonLiterals[jsonLitFalseQ : jsonLitFalseQ+7]
	// jsonLiteralNullQ  = jsonLiterals[jsonLitNullQ : jsonLitNullQ+6]

	jsonLiteralTrue  = jsonLiterals[jsonLitTrue : jsonLitTrue+4]
	jsonLiteralFalse = jsonLiterals[jsonLitFalse : jsonLitFalse+5]
	jsonLiteralNull  = jsonLiterals[jsonLitNull : jsonLitNull+4]

	// these are used, after consuming the first char
	jsonLiteral4True  = jsonLiterals[jsonLitTrue+1 : jsonLitTrue+4]
	jsonLiteral4False = jsonLiterals[jsonLitFalse+1 : jsonLitFalse+5]
	jsonLiteral4Null  = jsonLiterals[jsonLitNull+1 : jsonLitNull+4]
)

const (
	jsonU4Chk2 = '0'
	jsonU4Chk1 = 'a' - 10
	jsonU4Chk0 = 'A' - 10
)

const (
	// If !jsonValidateSymbols, decoding will be faster, by skipping some checks:
	//   - If we see first character of null, false or true,
	//     do not validate subsequent characters.
	//   - e.g. if we see a n, assume null and skip next 3 characters,
	//     and do not validate they are ull.
	// P.S. Do not expect a significant decoding boost from this.
	jsonValidateSymbols = true

	// jsonEscapeMultiByteUnicodeSep controls whether some unicode characters
	// that are valid json but may bomb in some contexts are escaped during encoeing.
	//
	// U+2028 is LINE SEPARATOR. U+2029 is PARAGRAPH SEPARATOR.
	// Both technically valid JSON, but bomb on JSONP, so fix here unconditionally.
	jsonEscapeMultiByteUnicodeSep = true

	// jsonManualInlineDecRdInHotZones controls whether we manually inline some decReader calls.
	//
	// encode performance is at par with libraries that just iterate over bytes directly,
	// because encWr (with inlined bytesEncAppender calls) is inlined.
	// Conversely, decode performance suffers because decRd (with inlined bytesDecReader calls)
	// isn't inlinable.
	//
	// To improve decode performamnce from json:
	// - readn1 is only called for \u
	// - consequently, to optimize json decoding, we specifically need inlining
	//   for bytes use-case of some other decReader methods:
	//   - jsonReadAsisChars, skipWhitespace (advance) and jsonReadNum
	//   - AND THEN readn3, readn4 (for ull, rue and alse).
	//   - (readn1 is only called when a char is escaped).
	// - without inlining, we still pay the cost of a method invocationK, and this dominates time
	// - To mitigate, we manually inline in hot zones
	//   *excluding places where used sparingly (e.g. nextValueBytes, and other atypical cases)*.
	//   - jsonReadAsisChars *only* called in: appendStringAsBytes
	//   - advance called: everywhere
	//   - jsonReadNum: decNumBytes, DecodeNaked
	// - From running go test (our anecdotal findings):
	//   - calling jsonReadAsisChars in appendStringAsBytes: 23431
	//   - calling jsonReadNum in decNumBytes: 15251
	//   - calling jsonReadNum in DecodeNaked: 612
	// Consequently, we manually inline jsonReadAsisChars (in appendStringAsBytes)
	// and jsonReadNum (in decNumbytes)
	jsonManualInlineDecRdInHotZones = true

	jsonSpacesOrTabsLen = 128

	jsonAlwaysReturnInternString = false
)

var (
	// jsonTabs and jsonSpaces are used as caches for indents
	jsonTabs, jsonSpaces [jsonSpacesOrTabsLen]byte

	jsonCharHtmlSafeSet bitset256
	jsonCharSafeSet     bitset256
)

func init() {
	var i byte
	for i = 0; i < jsonSpacesOrTabsLen; i++ {
		jsonSpaces[i] = ' '
		jsonTabs[i] = '\t'
	}

	// populate the safe values as true: note: ASCII control characters are (0-31)
	// jsonCharSafeSet:     all true except (0-31) " \
	// jsonCharHtmlSafeSet: all true except (0-31) " \ < > &
	for i = 32; i < utf8.RuneSelf; i++ {
		switch i {
		case '"', '\\':
		case '<', '>', '&':
			jsonCharSafeSet.set(i) // = true
		default:
			jsonCharSafeSet.set(i)
			jsonCharHtmlSafeSet.set(i)
		}
	}
}

// ----------------

type jsonEncDriver struct {
	noBuiltInTypes
	h *JsonHandle

	se interfaceExtWrapper

	// ---- cpu cache line boundary?
	di int8   // indent per: if negative, use tabs
	d  bool   // indenting?
	dl uint16 // indent level
	ks bool   // map key as string
	is byte   // integer as string

	typical bool

	s *bitset256 // safe set for characters (taking h.HTMLAsIs into consideration)

	// scratch: encode time, numbers, etc. Note: leave 1 byte for containerState
	b [cacheLineSize + 24]byte // buffer for encoding numbers and time

	e Encoder
}

func (e *jsonEncDriver) encoder() *Encoder { return &e.e }

func (e *jsonEncDriver) writeIndent() {
	e.e.encWr.writen1('\n')
	x := int(e.di) * int(e.dl)
	if e.di < 0 {
		x = -x
		for x > jsonSpacesOrTabsLen {
			e.e.encWr.writeb(jsonTabs[:])
			x -= jsonSpacesOrTabsLen
		}
		e.e.encWr.writeb(jsonTabs[:x])
	} else {
		for x > jsonSpacesOrTabsLen {
			e.e.encWr.writeb(jsonSpaces[:])
			x -= jsonSpacesOrTabsLen
		}
		e.e.encWr.writeb(jsonSpaces[:x])
	}
}

func (e *jsonEncDriver) WriteArrayElem() {
	if e.e.c != containerArrayStart {
		e.e.encWr.writen1(',')
	}
	if e.d {
		e.writeIndent()
	}
}

func (e *jsonEncDriver) WriteMapElemKey() {
	if e.e.c != containerMapStart {
		e.e.encWr.writen1(',')
	}
	if e.d {
		e.writeIndent()
	}
}

func (e *jsonEncDriver) WriteMapElemValue() {
	if e.d {
		e.e.encWr.writen2(':', ' ')
	} else {
		e.e.encWr.writen1(':')
	}
}

func (e *jsonEncDriver) EncodeNil() {
	// We always encode nil as just null (never in quotes)
	// This allows us to easily decode if a nil in the json stream
	// ie if initial token is n.

	// e.e.encWr.writeb(jsonLiteralNull)
	e.e.encWr.writen4([4]byte{'n', 'u', 'l', 'l'})
}

func (e *jsonEncDriver) EncodeTime(t time.Time) {
	// Do NOT use MarshalJSON, as it allocates internally.
	// instead, we call AppendFormat directly, using our scratch buffer (e.b)

	if t.IsZero() {
		e.EncodeNil()
	} else {
		e.b[0] = '"'
		b := fmtTime(t, e.b[1:1])
		e.b[len(b)+1] = '"'
		e.e.encWr.writeb(e.b[:len(b)+2])
	}
}

func (e *jsonEncDriver) EncodeExt(rv interface{}, xtag uint64, ext Ext) {
	if ext == SelfExt {
		rv2 := baseRV(rv)
		e.e.encodeValue(rv2, e.h.fnNoExt(rvType(rv2)))
	} else if v := ext.ConvertExt(rv); v == nil {
		e.EncodeNil()
	} else {
		e.e.encode(v)
	}
}

func (e *jsonEncDriver) EncodeRawExt(re *RawExt) {
	// only encodes re.Value (never re.Data)
	if re.Value == nil {
		e.EncodeNil()
	} else {
		e.e.encode(re.Value)
	}
}

func (e *jsonEncDriver) EncodeBool(b bool) {
	// Use writen with an array instead of writeb with a slice
	// i.e. in place of e.e.encWr.writeb(jsonLiteralTrueQ)
	//      OR jsonLiteralTrue, jsonLiteralFalse, jsonLiteralFalseQ, etc

	if e.ks && e.e.c == containerMapKey {
		if b {
			e.e.encWr.writen4([4]byte{'"', 't', 'r', 'u'})
			e.e.encWr.writen2('e', '"')
		} else {
			e.e.encWr.writen4([4]byte{'"', 'f', 'a', 'l'})
			e.e.encWr.writen2('s', 'e')
			e.e.encWr.writen1('"')
		}
	} else {
		if b {
			e.e.encWr.writen4([4]byte{'t', 'r', 'u', 'e'})
		} else {
			e.e.encWr.writen4([4]byte{'f', 'a', 'l', 's'})
			e.e.encWr.writen1('e')
		}
	}
}

func (e *jsonEncDriver) encodeFloat(f float64, bitsize, fmt byte, prec int8) {
	var blen uint
	if e.ks && e.e.c == containerMapKey {
		blen = 2 + uint(len(strconv.AppendFloat(e.b[1:1], f, fmt, int(prec), int(bitsize))))
		// _ = e.b[:blen]
		e.b[0] = '"'
		e.b[blen-1] = '"'
		e.e.encWr.writeb(e.b[:blen])
	} else {
		e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], f, fmt, int(prec), int(bitsize)))
	}
}

func (e *jsonEncDriver) EncodeFloat64(f float64) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		e.EncodeNil()
		return
	}
	fmt, prec := jsonFloatStrconvFmtPrec64(f)
	e.encodeFloat(f, 64, fmt, prec)
}

func (e *jsonEncDriver) EncodeFloat32(f float32) {
	if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
		e.EncodeNil()
		return
	}
	fmt, prec := jsonFloatStrconvFmtPrec32(f)
	e.encodeFloat(float64(f), 32, fmt, prec)
}

func (e *jsonEncDriver) encodeUint(neg bool, quotes bool, u uint64) {
	// copied mostly from std library: strconv
	// this should only be called on 64bit OS.

	const smallsString = "00010203040506070809" +
		"10111213141516171819" +
		"20212223242526272829" +
		"30313233343536373839" +
		"40414243444546474849" +
		"50515253545556575859" +
		"60616263646566676869" +
		"70717273747576777879" +
		"80818283848586878889" +
		"90919293949596979899"

	// typically, 19 or 20 bytes sufficient for decimal encoding a uint64
	// var a [24]byte
	var a = e.b[0:24]
	var i = uint8(len(a))

	if quotes {
		i--
		a[i] = '"'
	}
	// u guaranteed to fit into a uint (as we are not 32bit OS)
	var is uint
	var us = uint(u)
	for us >= 100 {
		is = us % 100 * 2
		us /= 100
		i -= 2
		a[i+1] = smallsString[is+1]
		a[i+0] = smallsString[is+0]
	}

	// us < 100
	is = us * 2
	i--
	a[i] = smallsString[is+1]
	if us >= 10 {
		i--
		a[i] = smallsString[is]
	}
	if neg {
		i--
		a[i] = '-'
	}
	if quotes {
		i--
		a[i] = '"'
	}
	e.e.encWr.writeb(a[i:])
}

func (e *jsonEncDriver) EncodeInt(v int64) {
	quotes := e.is == 'A' || e.is == 'L' && (v > 1<<53 || v < -(1<<53)) ||
		(e.ks && e.e.c == containerMapKey)

	if cpu32Bit {
		if quotes {
			blen := 2 + len(strconv.AppendInt(e.b[1:1], v, 10))
			e.b[0] = '"'
			e.b[blen-1] = '"'
			e.e.encWr.writeb(e.b[:blen])
		} else {
			e.e.encWr.writeb(strconv.AppendInt(e.b[:0], v, 10))
		}
		return
	}

	if v < 0 {
		e.encodeUint(true, quotes, uint64(-v))
	} else {
		e.encodeUint(false, quotes, uint64(v))
	}
}

func (e *jsonEncDriver) EncodeUint(v uint64) {
	quotes := e.is == 'A' || e.is == 'L' && v > 1<<53 || (e.ks && e.e.c == containerMapKey)

	if cpu32Bit {
		if quotes {
			blen := 2 + len(strconv.AppendUint(e.b[1:1], v, 10))
			e.b[0] = '"'
			e.b[blen-1] = '"'
			e.e.encWr.writeb(e.b[:blen])
		} else {
			e.e.encWr.writeb(strconv.AppendUint(e.b[:0], v, 10))
		}
		return
	}

	e.encodeUint(false, quotes, v)
}

func (e *jsonEncDriver) EncodeString(v string) {
	if e.h.StringToRaw {
		e.EncodeStringBytesRaw(bytesView(v))
		return
	}
	e.quoteStr(v)
}

func (e *jsonEncDriver) EncodeStringBytesRaw(v []byte) {
	// if encoding raw bytes and RawBytesExt is configured, use it to encode
	if v == nil {
		e.EncodeNil()
		return
	}
	if e.se.InterfaceExt != nil {
		e.EncodeExt(v, 0, &e.se)
		return
	}

	slen := base64.StdEncoding.EncodedLen(len(v)) + 2
	var bs []byte
	if len(e.b) < slen {
		bs = e.e.blist.get(slen)[:slen]
	} else {
		bs = e.b[:slen]
	}
	bs[0] = '"'
	base64.StdEncoding.Encode(bs[1:], v)
	bs[len(bs)-1] = '"'
	e.e.encWr.writeb(bs)
	if len(e.b) < slen {
		e.e.blist.put(bs)
	}
}

// indent is done as below:
//   - newline and indent are added before each mapKey or arrayElem
//   - newline and indent are added before each ending,
//     except there was no entry (so we can have {} or [])

func (e *jsonEncDriver) WriteArrayStart(length int) {
	if e.d {
		e.dl++
	}
	e.e.encWr.writen1('[')
}

func (e *jsonEncDriver) WriteArrayEnd() {
	if e.d {
		e.dl--
		e.writeIndent()
	}
	e.e.encWr.writen1(']')
}

func (e *jsonEncDriver) WriteMapStart(length int) {
	if e.d {
		e.dl++
	}
	e.e.encWr.writen1('{')
}

func (e *jsonEncDriver) WriteMapEnd() {
	if e.d {
		e.dl--
		if e.e.c != containerMapStart {
			e.writeIndent()
		}
	}
	e.e.encWr.writen1('}')
}

func (e *jsonEncDriver) quoteStr(s string) {
	// adapted from std pkg encoding/json
	const hex = "0123456789abcdef"
	w := e.e.w()
	w.writen1('"')
	var i, start uint
	for i < uint(len(s)) {
		// encode all bytes < 0x20 (except \r, \n).
		// also encode < > & to prevent security holes when served to some browsers.

		// We optimize for ascii, by assumining that most characters are in the BMP
		// and natively consumed by json without much computation.

		// if 0x20 <= b && b != '\\' && b != '"' && b != '<' && b != '>' && b != '&' {
		// if (htmlasis && jsonCharSafeSet.isset(b)) || jsonCharHtmlSafeSet.isset(b) {
		if e.s.isset(s[i]) {
			i++
			continue
		}
		// b := s[i]
		if s[i] < utf8.RuneSelf {
			if start < i {
				w.writestr(s[start:i])
			}
			switch s[i] {
			case '\\', '"':
				w.writen2('\\', s[i])
			case '\n':
				w.writen2('\\', 'n')
			case '\r':
				w.writen2('\\', 'r')
			case '\b':
				w.writen2('\\', 'b')
			case '\f':
				w.writen2('\\', 'f')
			case '\t':
				w.writen2('\\', 't')
			default:
				w.writestr(`\u00`)
				w.writen2(hex[s[i]>>4], hex[s[i]&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 { // meaning invalid encoding (so output as-is)
			if start < i {
				w.writestr(s[start:i])
			}
			w.writestr(`\uFFFD`)
			i++
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR. U+2029 is PARAGRAPH SEPARATOR.
		// Both technically valid JSON, but bomb on JSONP, so fix here *unconditionally*.
		if jsonEscapeMultiByteUnicodeSep && (c == '\u2028' || c == '\u2029') {
			if start < i {
				w.writestr(s[start:i])
			}
			w.writestr(`\u202`)
			w.writen1(hex[c&0xF])
			i += uint(size)
			start = i
			continue
		}
		i += uint(size)
	}
	if start < uint(len(s)) {
		w.writestr(s[start:])
	}
	w.writen1('"')
}

func (e *jsonEncDriver) atEndOfEncode() {
	if e.h.TermWhitespace {
		if e.e.c == 0 { // scalar written, output space
			e.e.encWr.writen1(' ')
		} else { // container written, output new-line
			e.e.encWr.writen1('\n')
		}
	}
}

// ----------

type jsonDecDriver struct {
	noBuiltInTypes
	h *JsonHandle

	tok  uint8   // used to store the token read right after skipWhiteSpace
	_    bool    // found null
	_    [2]byte // padding
	bstr [4]byte // scratch used for string \UXXX parsing

	buf []byte
	se  interfaceExtWrapper

	// ---- cpu cache line boundary?

	d Decoder
}

func (d *jsonDecDriver) decoder() *Decoder {
	return &d.d
}

func (d *jsonDecDriver) ReadMapStart() int {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return containerLenNil
	}
	if d.tok != '{' {
		d.d.errorf("read map - expect char '%c' but got char '%c'", '{', d.tok)
	}
	d.tok = 0
	return containerLenUnknown
}

func (d *jsonDecDriver) ReadArrayStart() int {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return containerLenNil
	}
	if d.tok != '[' {
		d.d.errorf("read array - expect char '%c' but got char '%c'", '[', d.tok)
	}
	d.tok = 0
	return containerLenUnknown
}

func (d *jsonDecDriver) CheckBreak() bool {
	d.advance()
	return d.tok == '}' || d.tok == ']'
}

func (d *jsonDecDriver) ReadArrayElem() {
	const xc uint8 = ','
	if d.d.c != containerArrayStart {
		d.advance()
		if d.tok != xc {
			d.readDelimError(xc)
		}
		d.tok = 0
	}
}

func (d *jsonDecDriver) ReadArrayEnd() {
	const xc uint8 = ']'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

func (d *jsonDecDriver) ReadMapElemKey() {
	const xc uint8 = ','
	if d.d.c != containerMapStart {
		d.advance()
		if d.tok != xc {
			d.readDelimError(xc)
		}
		d.tok = 0
	}
}

func (d *jsonDecDriver) ReadMapElemValue() {
	const xc uint8 = ':'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

func (d *jsonDecDriver) ReadMapEnd() {
	const xc uint8 = '}'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

func (d *jsonDecDriver) readDelimError(xc uint8) {
	d.d.errorf("read json delimiter - expect char '%c' but got char '%c'", xc, d.tok)
}

func (d *jsonDecDriver) readLit4True() {
	bs := d.d.decRd.readn3()
	d.tok = 0
	if jsonValidateSymbols && bs != [...]byte{'r', 'u', 'e'} { // !Equal jsonLiteral4True
		d.d.errorf("expecting %s: got %s", jsonLiteral4True, bs[:])
	}
}

func (d *jsonDecDriver) readLit4False() {
	bs := d.d.decRd.readn4()
	d.tok = 0
	if jsonValidateSymbols && bs != [...]byte{'a', 'l', 's', 'e'} { // !Equal jsonLiteral4False
		d.d.errorf("expecting %s: got %s", jsonLiteral4False, bs[:])
	}
}

func (d *jsonDecDriver) readLit4Null() {
	bs := d.d.decRd.readn3() // readx(3)
	d.tok = 0
	if jsonValidateSymbols && bs != [...]byte{'u', 'l', 'l'} { // !Equal jsonLiteral4Null
		d.d.errorf("expecting %s: got %s", jsonLiteral4Null, bs[:])
	}
}

func (d *jsonDecDriver) advance() {
	if d.tok == 0 {
		d.tok = d.d.decRd.skipWhitespace() // skip(&whitespaceCharBitset)
	}
}

func (d *jsonDecDriver) nextValueBytes(start []byte) (v []byte) {
	v = start
	dr := &d.d.decRd
	consumeString := func() {
		for {
			bs := dr.jsonReadAsisChars()
			v = append(v, bs...)
			if bs[len(bs)-1] == '"' {
				break
			}
			// last char is '\', so consume next one
			v = append(v, dr.readn1())
		}
	}

	d.advance() // ignore leading whitespace

	switch d.tok {
	default:
		v = append(v, dr.jsonReadNum()...)
	case 'n':
		d.readLit4Null()
		v = append(v, jsonLiteralNull...)
	case 'f':
		d.readLit4False()
		v = append(v, jsonLiteralFalse...)
	case 't':
		d.readLit4True()
		v = append(v, jsonLiteralTrue...)
	case '"':
		v = append(v, '"')
		consumeString()
	case '{', '[':
		var elem struct{}
		var stack []struct{}

		stack = append(stack, elem)

		v = append(v, d.tok)

		for len(stack) != 0 {
			c := dr.readn1()
			v = append(v, c)
			switch c {
			case '"':
				consumeString()
			case '{', '[':
				stack = append(stack, elem)
			case '}', ']':
				stack = stack[:len(stack)-1]
			}
		}
	}
	d.tok = 0
	return
}

func (d *jsonDecDriver) TryNil() bool {
	d.advance()
	// we shouldn't try to see if quoted "null" was here, right?
	// only the plain string: `null` denotes a nil (ie not quotes)
	if d.tok == 'n' {
		d.readLit4Null()
		return true
	}
	return false
}

func (d *jsonDecDriver) DecodeBool() (v bool) {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	fquot := d.d.c == containerMapKey && d.tok == '"'
	if fquot {
		d.tok = d.d.decRd.readn1()
	}
	switch d.tok {
	case 'f':
		d.readLit4False()
		// v = false
	case 't':
		d.readLit4True()
		v = true
	default:
		d.d.errorf("decode bool: got first char %c", d.tok)
		// v = false // "unreachable"
	}
	if fquot {
		d.d.decRd.readn1()
	}
	return
}

func (d *jsonDecDriver) DecodeTime() (t time.Time) {
	// read string, and pass the string into json.unmarshal
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	bs := d.readUnescapedString()
	t, err := time.Parse(time.RFC3339, stringView(bs))
	d.d.onerror(err)
	return
}

func (d *jsonDecDriver) ContainerType() (vt valueType) {
	// check container type by checking the first char
	d.advance()

	// optimize this, so we don't do 4 checks but do one computation.
	// return jsonContainerSet[d.tok]

	// ContainerType is mostly called for Map and Array,
	// so this conditional is good enough (max 2 checks typically)
	if d.tok == '{' {
		return valueTypeMap
	} else if d.tok == '[' {
		return valueTypeArray
	} else if d.tok == 'n' {
		d.readLit4Null()
		return valueTypeNil
	} else if d.tok == '"' {
		return valueTypeString
	}
	return valueTypeUnset
}

func (d *jsonDecDriver) decNumBytes() (bs []byte) {
	d.advance()
	dr := &d.d.decRd
	if d.tok == '"' {
		bs = dr.readUntil('"')
	} else if d.tok == 'n' {
		d.readLit4Null()
	} else {
		if jsonManualInlineDecRdInHotZones {
			if dr.bytes {
				bs = dr.rb.jsonReadNum()
			} else if dr.bufio {
				bs = dr.bi.jsonReadNum()
			} else {
				bs = dr.ri.jsonReadNum()
			}
		} else {
			bs = dr.jsonReadNum()
		}
	}
	d.tok = 0
	return
}

func (d *jsonDecDriver) DecodeUint64() (u uint64) {
	bs := d.decNumBytes()
	if len(bs) == 0 {
		return
	}
	if bs[0] == '-' {
		d.d.errorf("negative number cannot be decoded as uint64")
	}
	var r readFloatResult
	u, r.ok = parseUint64_simple(bs)
	if r.ok {
		return
	}

	r = readFloat(bs, fi64u)
	if r.ok {
		u, r.bad = parseUint64_reader(r)
		if r.bad {
			d.d.onerror(strconvParseErr(bs, "ParseUint"))
		}
		return
	}
	d.d.onerror(strconvParseErr(bs, "ParseUint"))
	return
}

func (d *jsonDecDriver) DecodeInt64() (v int64) {
	b := d.decNumBytes()
	if len(b) == 0 {
		return
	}

	var r readFloatResult
	var neg bool

	if b[0] == '-' {
		neg = true
		b = b[1:]
	}

	r.mantissa, r.ok = parseUint64_simple(b)
	if r.ok {
		if chkOvf.Uint2Int(r.mantissa, neg) {
			d.d.errorf("overflow decoding number from %s", b)
		}
		if neg {
			v = -int64(r.mantissa)
		} else {
			v = int64(r.mantissa)
		}
		return
	}

	r = readFloat(b, fi64i)
	if r.ok {
		r.neg = neg
		v, r.bad = parseInt64_reader(r)
		if r.bad {
			d.d.onerror(strconvParseErr(b, "ParseInt"))
		}
		return
	}
	d.d.onerror(strconvParseErr(b, "ParseInt"))
	return
}

func (d *jsonDecDriver) DecodeFloat64() (f float64) {
	var err error
	bs := d.decNumBytes()
	if len(bs) == 0 {
		return
	}
	f, err = parseFloat64(bs)
	d.d.onerror(err)
	return
}

func (d *jsonDecDriver) DecodeFloat32() (f float32) {
	var err error
	bs := d.decNumBytes()
	if len(bs) == 0 {
		return
	}
	f, err = parseFloat32(bs)
	d.d.onerror(err)
	return
}

func (d *jsonDecDriver) DecodeExt(rv interface{}, xtag uint64, ext Ext) {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	if ext == nil {
		re := rv.(*RawExt)
		re.Tag = xtag
		d.d.decode(&re.Value)
	} else if ext == SelfExt {
		rv2 := baseRV(rv)
		d.d.decodeValue(rv2, d.h.fnNoExt(rvType(rv2)))
	} else {
		d.d.interfaceExtConvertAndDecode(rv, ext)
	}
}

func (d *jsonDecDriver) decBytesFromArray(bs []byte) []byte {
	if bs == nil {
		bs = []byte{}
	} else {
		bs = bs[:0]
	}
	d.tok = 0
	bs = append(bs, uint8(d.DecodeUint64()))
	d.tok = d.d.decRd.skipWhitespace() // skip(&whitespaceCharBitset)
	for d.tok != ']' {
		if d.tok != ',' {
			d.d.errorf("read array element - expect char '%c' but got char '%c'", ',', d.tok)
		}
		d.tok = 0
		bs = append(bs, uint8(chkOvf.UintV(d.DecodeUint64(), 8)))
		d.tok = d.d.decRd.skipWhitespace() // skip(&whitespaceCharBitset)
	}
	d.tok = 0
	return bs
}

func (d *jsonDecDriver) DecodeBytes(bs []byte, zerocopy bool) (bsOut []byte) {
	// if decoding into raw bytes, and the RawBytesExt is configured, use it to decode.
	if d.se.InterfaceExt != nil {
		bsOut = bs
		d.DecodeExt(&bsOut, 0, &d.se)
		return
	}
	d.advance()
	// check if an "array" of uint8's (see ContainerType for how to infer if an array)
	if d.tok == '[' {
		// bsOut, _ = fastpathTV.DecSliceUint8V(bs, true, d.d)
		if zerocopy && len(bs) == 0 {
			bs = d.d.b[:]
		}
		return d.decBytesFromArray(bs)
	}

	// base64 encodes []byte{} as "", and we encode nil []byte as null.
	// Consequently, base64 should decode null as a nil []byte, and "" as an empty []byte{}.
	// appendStringAsBytes returns a zero-len slice for both, so as not to reset d.buf.

	if d.tok == 'n' {
		d.readLit4Null()
		return nil
	}

	bs1 := d.readUnescapedString()
	slen := base64.StdEncoding.DecodedLen(len(bs1))
	if slen == 0 {
		bsOut = []byte{}
	} else if slen <= cap(bs) {
		bsOut = bs[:slen]
	} else if zerocopy {
		d.buf = d.d.blist.check(d.buf, slen)[:slen]
		bsOut = d.buf
	} else {
		bsOut = make([]byte, slen)
	}
	slen2, err := base64.StdEncoding.Decode(bsOut, bs1)
	if err != nil {
		d.d.errorf("error decoding base64 binary '%s': %v", bs1, err)
	}
	if slen != slen2 {
		bsOut = bsOut[:slen2]
	}
	return
}

func (d *jsonDecDriver) DecodeStringAsBytes() (s []byte) {
	d.advance()

	// common case
	if d.tok == '"' {
		d.appendStringAsBytes()
		return d.buf
	}

	// handle non-string scalar: null, true, false or a number
	switch d.tok {
	case 'n':
		d.readLit4Null()
		return nil // []byte{}
	case 'f':
		d.readLit4False()
		return jsonLiteralFalse
	case 't':
		d.readLit4True()
		return jsonLiteralTrue
	}

	// try to parse a valid number
	d.tok = 0
	return d.d.decRd.jsonReadNum()
}

func (d *jsonDecDriver) readUnescapedString() (bs []byte) {
	if d.tok != '"' {
		d.d.errorf("expecting string starting with '\"'; got '%c'", d.tok)
	}

	bs = d.d.decRd.readUntil('"')
	d.tok = 0
	return
}

func (d *jsonDecDriver) appendStringAsBytes() {
	// use a local buf variable, so we don't do pointer chasing within loop
	buf := d.buf[:0]
	dr := &d.d.decRd
	d.tok = 0

	var bs []byte
	var c byte
	for {
		if jsonManualInlineDecRdInHotZones {
			if dr.bytes {
				bs = dr.rb.jsonReadAsisChars()
			} else if dr.bufio {
				bs = dr.bi.jsonReadAsisChars()
			} else {
				bs = dr.ri.jsonReadAsisChars()
			}
		} else {
			bs = dr.jsonReadAsisChars()
		}

		buf = append(buf, bs[:len(bs)-1]...)
		c = bs[len(bs)-1]

		if c == '"' {
			break
		}

		// c is now '\'
		c = dr.readn1()

		switch c {
		case '"', '\\', '/', '\'':
			buf = append(buf, c)
		case 'b':
			buf = append(buf, '\b')
		case 'f':
			buf = append(buf, '\f')
		case 'n':
			buf = append(buf, '\n')
		case 'r':
			buf = append(buf, '\r')
		case 't':
			buf = append(buf, '\t')
		case 'u':
			buf = append(buf, d.bstr[:utf8.EncodeRune(d.bstr[:], d.appendStringAsBytesSlashU())]...)
		default:
			d.buf = buf
			d.d.errorf("unsupported escaped value: %c", c)
		}
	}
	d.buf = buf
}

func (d *jsonDecDriver) appendStringAsBytesSlashU() (r rune) {
	var rr uint32
	var csu [2]byte
	var cs [4]byte = d.d.decRd.readn4()
	if rr = jsonSlashURune(cs); rr == unicode.ReplacementChar {
		return unicode.ReplacementChar
	}
	r = rune(rr)
	if utf16.IsSurrogate(r) {
		csu = d.d.decRd.readn2()
		cs = d.d.decRd.readn4()
		if csu[0] == '\\' && csu[1] == 'u' {
			if rr = jsonSlashURune(cs); rr == unicode.ReplacementChar {
				return unicode.ReplacementChar
			}
			return utf16.DecodeRune(r, rune(rr))
		}
		return unicode.ReplacementChar
	}
	return
}

func jsonSlashURune(cs [4]byte) (rr uint32) {
	for _, c := range cs {
		// best to use explicit if-else
		// - not a table, etc which involve memory loads, array lookup with bounds checks, etc
		if c >= '0' && c <= '9' {
			rr = rr*16 + uint32(c-jsonU4Chk2)
		} else if c >= 'a' && c <= 'f' {
			rr = rr*16 + uint32(c-jsonU4Chk1)
		} else if c >= 'A' && c <= 'F' {
			rr = rr*16 + uint32(c-jsonU4Chk0)
		} else {
			return unicode.ReplacementChar
		}
	}
	return
}

func (d *jsonDecDriver) nakedNum(z *fauxUnion, bs []byte) (err error) {
	// Note: nakedNum is NEVER called with a zero-length []byte
	if d.h.PreferFloat {
		z.v = valueTypeFloat
		z.f, err = parseFloat64(bs)
	} else {
		err = parseNumber(bs, z, d.h.SignedInteger)
	}
	return
}

func (d *jsonDecDriver) sliceToString(bs []byte) string {
	if d.d.is != nil && (jsonAlwaysReturnInternString || d.d.c == containerMapKey) {
		return d.d.string(bs)
	}
	return string(bs)
}

func (d *jsonDecDriver) DecodeNaked() {
	z := d.d.naked()

	d.advance()
	var bs []byte
	switch d.tok {
	case 'n':
		d.readLit4Null()
		z.v = valueTypeNil
	case 'f':
		d.readLit4False()
		z.v = valueTypeBool
		z.b = false
	case 't':
		d.readLit4True()
		z.v = valueTypeBool
		z.b = true
	case '{':
		z.v = valueTypeMap // don't consume. kInterfaceNaked will call ReadMapStart
	case '[':
		z.v = valueTypeArray // don't consume. kInterfaceNaked will call ReadArrayStart
	case '"':
		// if a string, and MapKeyAsString, then try to decode it as a nil, bool or number first
		d.appendStringAsBytes()
		bs = d.buf
		if len(bs) > 0 && d.d.c == containerMapKey && d.h.MapKeyAsString {
			if bytes.Equal(bs, jsonLiteralNull) {
				z.v = valueTypeNil
			} else if bytes.Equal(bs, jsonLiteralTrue) {
				z.v = valueTypeBool
				z.b = true
			} else if bytes.Equal(bs, jsonLiteralFalse) {
				z.v = valueTypeBool
				z.b = false
			} else {
				// check if a number: float, int or uint
				if err := d.nakedNum(z, bs); err != nil {
					z.v = valueTypeString
					z.s = d.sliceToString(bs)
				}
			}
		} else {
			z.v = valueTypeString
			z.s = d.sliceToString(bs)
		}
	default: // number
		bs = d.d.decRd.jsonReadNum()
		d.tok = 0
		if len(bs) == 0 {
			d.d.errorf("decode number from empty string")
		}
		if err := d.nakedNum(z, bs); err != nil {
			d.d.errorf("decode number from %s: %v", bs, err)
		}
	}
}

//----------------------

// JsonHandle is a handle for JSON encoding format.
//
// Json is comprehensively supported:
//    - decodes numbers into interface{} as int, uint or float64
//      based on how the number looks and some config parameters e.g. PreferFloat, SignedInt, etc.
//    - decode integers from float formatted numbers e.g. 1.27e+8
//    - decode any json value (numbers, bool, etc) from quoted strings
//    - configurable way to encode/decode []byte .
//      by default, encodes and decodes []byte using base64 Std Encoding
//    - UTF-8 support for encoding and decoding
//
// It has better performance than the json library in the standard library,
// by leveraging the performance improvements of the codec library.
//
// In addition, it doesn't read more bytes than necessary during a decode, which allows
// reading multiple values from a stream containing json and non-json content.
// For example, a user can read a json value, then a cbor value, then a msgpack value,
// all from the same stream in sequence.
//
// Note that, when decoding quoted strings, invalid UTF-8 or invalid UTF-16 surrogate pairs are
// not treated as an error. Instead, they are replaced by the Unicode replacement character U+FFFD.
//
// Note also that the float values for NaN, +Inf or -Inf are encoded as null,
// as suggested by NOTE 4 of the ECMA-262 ECMAScript Language Specification 5.1 edition.
// see http://www.ecma-international.org/publications/files/ECMA-ST/Ecma-262.pdf .
type JsonHandle struct {
	textEncodingType
	BasicHandle

	// Indent indicates how a value is encoded.
	//   - If positive, indent by that number of spaces.
	//   - If negative, indent by that number of tabs.
	Indent int8

	// IntegerAsString controls how integers (signed and unsigned) are encoded.
	//
	// Per the JSON Spec, JSON numbers are 64-bit floating point numbers.
	// Consequently, integers > 2^53 cannot be represented as a JSON number without losing precision.
	// This can be mitigated by configuring how to encode integers.
	//
	// IntegerAsString interpretes the following values:
	//   - if 'L', then encode integers > 2^53 as a json string.
	//   - if 'A', then encode all integers as a json string
	//             containing the exact integer representation as a decimal.
	//   - else    encode all integers as a json number (default)
	IntegerAsString byte

	// HTMLCharsAsIs controls how to encode some special characters to html: < > &
	//
	// By default, we encode them as \uXXX
	// to prevent security holes when served from some browsers.
	HTMLCharsAsIs bool

	// PreferFloat says that we will default to decoding a number as a float.
	// If not set, we will examine the characters of the number and decode as an
	// integer type if it doesn't have any of the characters [.eE].
	PreferFloat bool

	// TermWhitespace says that we add a whitespace character
	// at the end of an encoding.
	//
	// The whitespace is important, especially if using numbers in a context
	// where multiple items are written to a stream.
	TermWhitespace bool

	// MapKeyAsString says to encode all map keys as strings.
	//
	// Use this to enforce strict json output.
	// The only caveat is that nil value is ALWAYS written as null (never as "null")
	MapKeyAsString bool

	// _ uint64 // padding (cache line)

	// Note: below, we store hardly-used items
	// e.g. RawBytesExt (which is already cached in the (en|de)cDriver).

	// RawBytesExt, if configured, is used to encode and decode raw bytes in a custom way.
	// If not configured, raw bytes are encoded to/from base64 text.
	RawBytesExt InterfaceExt

	// _ [5]uint64 // padding (cache line)
}

// Name returns the name of the handle: json
func (h *JsonHandle) Name() string { return "json" }

func (h *JsonHandle) desc(bd byte) string { return string(bd) }

// func (h *JsonHandle) hasElemSeparators() bool { return true }
func (h *JsonHandle) typical() bool {
	return h.Indent == 0 && !h.MapKeyAsString && h.IntegerAsString != 'A' && h.IntegerAsString != 'L'
}

func (h *JsonHandle) newEncDriver() encDriver {
	var e = &jsonEncDriver{h: h}
	e.e.e = e
	e.e.js = true
	e.e.init(h)
	e.reset()
	return e
}

func (h *JsonHandle) newDecDriver() decDriver {
	var d = &jsonDecDriver{h: h}
	d.d.d = d
	d.d.js = true
	d.d.jsms = h.MapKeyAsString
	d.d.init(h)
	d.reset()
	return d
}

func (e *jsonEncDriver) reset() {
	// (htmlasis && jsonCharSafeSet.isset(b)) || jsonCharHtmlSafeSet.isset(b)
	e.typical = e.h.typical()
	if e.h.HTMLCharsAsIs {
		e.s = &jsonCharSafeSet
	} else {
		e.s = &jsonCharHtmlSafeSet
	}
	e.se.InterfaceExt = e.h.RawBytesExt
	e.d, e.dl, e.di = false, 0, 0
	if e.h.Indent != 0 {
		e.d = true
		e.di = int8(e.h.Indent)
	}
	e.ks = e.h.MapKeyAsString
	e.is = e.h.IntegerAsString
}

func (d *jsonDecDriver) reset() {
	d.se.InterfaceExt = d.h.RawBytesExt
	d.buf = d.d.blist.check(d.buf, 256)
	d.tok = 0
}

func (d *jsonDecDriver) atEndOfDecode() {}

func jsonFloatStrconvFmtPrec64(f float64) (fmt byte, prec int8) {
	fmt = 'f'
	prec = -1
	fbits := math.Float64bits(f)
	abs := math.Float64frombits(fbits &^ (1 << 63))
	if abs == 0 || abs == 1 {
		prec = 1
	} else if abs < 1e-6 || abs >= 1e21 {
		fmt = 'e'
	} else {
		exp := uint64(fbits>>52)&0x7FF - 1023 // uint(x>>shift)&mask - bias
		// clear top 12+e bits, the integer part; if the rest is 0, then no fraction.
		if exp < 52 && fbits<<(12+exp) == 0 { // means there's no fractional part
			prec = 1
		}
	}
	return
}

func jsonFloatStrconvFmtPrec32(f float32) (fmt byte, prec int8) {
	fmt = 'f'
	prec = -1
	// directly handle Modf (to get fractions) and Abs (to get absolute)
	fbits := math.Float32bits(f)
	abs := math.Float32frombits(fbits &^ (1 << 31))
	if abs == 0 || abs == 1 {
		prec = 1
	} else if abs < 1e-6 || abs >= 1e21 {
		fmt = 'e'
	} else {
		exp := uint32(fbits>>23)&0xFF - 127 // uint(x>>shift)&mask - bias
		// clear top 9+e bits, the integer part; if the rest is 0, then no fraction.
		if exp < 23 && fbits<<(9+exp) == 0 { // means there's no fractional part
			prec = 1
		}
	}
	return
}

var _ decDriverContainerTracker = (*jsonDecDriver)(nil)
var _ encDriverContainerTracker = (*jsonEncDriver)(nil)
var _ decDriver = (*jsonDecDriver)(nil)

var _ encDriver = (*jsonEncDriver)(nil)
