package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	WhitespaceChars = " \f\n\r\t\v\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"
	Re2Dot          = "[^\r\n\u2028\u2029]"
)

type regexpParseError struct {
	offset int
	err    string
}

type RegexpErrorIncompatible struct {
	regexpParseError
}
type RegexpSyntaxError struct {
	regexpParseError
}

func (s regexpParseError) Error() string {
	return s.err
}

type _RegExp_parser struct {
	str    string
	length int

	chr       rune // The current character
	chrOffset int  // The offset of current character
	offset    int  // The offset after current character (may be greater than 1)

	err error

	goRegexp   strings.Builder
	passOffset int
}

// TransformRegExp transforms a JavaScript pattern into  a Go "regexp" pattern.
//
// re2 (Go) cannot do backtracking, so the presence of a lookahead (?=) (?!) or
// backreference (\1, \2, ...) will cause an error.
//
// re2 (Go) has a different definition for \s: [\t\n\f\r ].
// The JavaScript definition, on the other hand, also includes \v, Unicode "Separator, Space", etc.
//
// If the pattern is valid, but incompatible (contains a lookahead or backreference),
// then this function returns an empty string an error of type RegexpErrorIncompatible.
//
// If the pattern is invalid (not valid even in JavaScript), then this function
// returns an empty string and a generic error.
func TransformRegExp(pattern string) (transformed string, err error) {

	if pattern == "" {
		return "", nil
	}

	parser := _RegExp_parser{
		str:    pattern,
		length: len(pattern),
	}
	err = parser.parse()
	if err != nil {
		return "", err
	}

	return parser.ResultString(), nil
}

func (self *_RegExp_parser) ResultString() string {
	if self.passOffset != -1 {
		return self.str[:self.passOffset]
	}
	return self.goRegexp.String()
}

func (self *_RegExp_parser) parse() (err error) {
	self.read() // Pull in the first character
	self.scan()
	return self.err
}

func (self *_RegExp_parser) read() {
	if self.offset < self.length {
		self.chrOffset = self.offset
		chr, width := rune(self.str[self.offset]), 1
		if chr >= utf8.RuneSelf { // !ASCII
			chr, width = utf8.DecodeRuneInString(self.str[self.offset:])
			if chr == utf8.RuneError && width == 1 {
				self.error(true, "Invalid UTF-8 character")
				return
			}
		}
		self.offset += width
		self.chr = chr
	} else {
		self.chrOffset = self.length
		self.chr = -1 // EOF
	}
}

func (self *_RegExp_parser) stopPassing() {
	self.goRegexp.Grow(3 * len(self.str) / 2)
	self.goRegexp.WriteString(self.str[:self.passOffset])
	self.passOffset = -1
}

func (self *_RegExp_parser) write(p []byte) {
	if self.passOffset != -1 {
		self.stopPassing()
	}
	self.goRegexp.Write(p)
}

func (self *_RegExp_parser) writeByte(b byte) {
	if self.passOffset != -1 {
		self.stopPassing()
	}
	self.goRegexp.WriteByte(b)
}

func (self *_RegExp_parser) writeString(s string) {
	if self.passOffset != -1 {
		self.stopPassing()
	}
	self.goRegexp.WriteString(s)
}

func (self *_RegExp_parser) scan() {
	for self.chr != -1 {
		switch self.chr {
		case '\\':
			self.read()
			self.scanEscape(false)
		case '(':
			self.pass()
			self.scanGroup()
		case '[':
			self.scanBracket()
		case ')':
			self.error(true, "Unmatched ')'")
			return
		case '.':
			self.writeString(Re2Dot)
			self.read()
		default:
			self.pass()
		}
	}
}

// (...)
func (self *_RegExp_parser) scanGroup() {
	str := self.str[self.chrOffset:]
	if len(str) > 1 { // A possibility of (?= or (?!
		if str[0] == '?' {
			ch := str[1]
			switch {
			case ch == '=' || ch == '!':
				self.error(false, "re2: Invalid (%s) <lookahead>", self.str[self.chrOffset:self.chrOffset+2])
				return
			case ch == '<':
				self.error(false, "re2: Invalid (%s) <lookbehind>", self.str[self.chrOffset:self.chrOffset+2])
				return
			case ch != ':':
				self.error(true, "Invalid group")
				return
			}
		}
	}
	for self.chr != -1 && self.chr != ')' {
		switch self.chr {
		case '\\':
			self.read()
			self.scanEscape(false)
		case '(':
			self.pass()
			self.scanGroup()
		case '[':
			self.scanBracket()
		case '.':
			self.writeString(Re2Dot)
			self.read()
		default:
			self.pass()
			continue
		}
	}
	if self.chr != ')' {
		self.error(true, "Unterminated group")
		return
	}
	self.pass()
}

// [...]
func (self *_RegExp_parser) scanBracket() {
	str := self.str[self.chrOffset:]
	if strings.HasPrefix(str, "[]") {
		// [] -- Empty character class
		self.writeString("[^\u0000-\U0001FFFF]")
		self.offset += 1
		self.read()
		return
	}

	if strings.HasPrefix(str, "[^]") {
		self.writeString("[\u0000-\U0001FFFF]")
		self.offset += 2
		self.read()
		return
	}

	self.pass()
	for self.chr != -1 {
		if self.chr == ']' {
			break
		} else if self.chr == '\\' {
			self.read()
			self.scanEscape(true)
			continue
		}
		self.pass()
	}
	if self.chr != ']' {
		self.error(true, "Unterminated character class")
		return
	}
	self.pass()
}

// \...
func (self *_RegExp_parser) scanEscape(inClass bool) {
	offset := self.chrOffset

	var length, base uint32
	switch self.chr {

	case '0', '1', '2', '3', '4', '5', '6', '7':
		var value int64
		size := 0
		for {
			digit := int64(digitValue(self.chr))
			if digit >= 8 {
				// Not a valid digit
				break
			}
			value = value*8 + digit
			self.read()
			size += 1
		}
		if size == 1 { // The number of characters read
			if value != 0 {
				// An invalid backreference
				self.error(false, "re2: Invalid \\%d <backreference>", value)
				return
			}
			self.passString(offset-1, self.chrOffset)
			return
		}
		tmp := []byte{'\\', 'x', '0', 0}
		if value >= 16 {
			tmp = tmp[0:2]
		} else {
			tmp = tmp[0:3]
		}
		tmp = strconv.AppendInt(tmp, value, 16)
		self.write(tmp)
		return

	case '8', '9':
		self.read()
		self.error(false, "re2: Invalid \\%s <backreference>", self.str[offset:self.chrOffset])
		return

	case 'x':
		self.read()
		length, base = 2, 16

	case 'u':
		self.read()
		if self.chr == '{' {
			self.read()
			length, base = 0, 16
		} else {
			length, base = 4, 16
		}

	case 'b':
		if inClass {
			self.write([]byte{'\\', 'x', '0', '8'})
			self.read()
			return
		}
		fallthrough

	case 'B':
		fallthrough

	case 'd', 'D', 'w', 'W':
		// This is slightly broken, because ECMAScript
		// includes \v in \s, \S, while re2 does not
		fallthrough

	case '\\':
		fallthrough

	case 'f', 'n', 'r', 't', 'v':
		self.passString(offset-1, self.offset)
		self.read()
		return

	case 'c':
		self.read()
		var value int64
		if 'a' <= self.chr && self.chr <= 'z' {
			value = int64(self.chr - 'a' + 1)
		} else if 'A' <= self.chr && self.chr <= 'Z' {
			value = int64(self.chr - 'A' + 1)
		} else {
			self.writeByte('c')
			return
		}
		tmp := []byte{'\\', 'x', '0', 0}
		if value >= 16 {
			tmp = tmp[0:2]
		} else {
			tmp = tmp[0:3]
		}
		tmp = strconv.AppendInt(tmp, value, 16)
		self.write(tmp)
		self.read()
		return
	case 's':
		if inClass {
			self.writeString(WhitespaceChars)
		} else {
			self.writeString("[" + WhitespaceChars + "]")
		}
		self.read()
		return
	case 'S':
		if inClass {
			self.error(false, "S in class")
			return
		} else {
			self.writeString("[^" + WhitespaceChars + "]")
		}
		self.read()
		return
	default:
		// $ is an identifier character, so we have to have
		// a special case for it here
		if self.chr == '$' || self.chr < utf8.RuneSelf && !isIdentifierPart(self.chr) {
			// A non-identifier character needs escaping
			self.passString(offset-1, self.offset)
			self.read()
			return
		}
		// Unescape the character for re2
		self.pass()
		return
	}

	// Otherwise, we're a \u.... or \x...
	valueOffset := self.chrOffset

	if length > 0 {
		for length := length; length > 0; length-- {
			digit := uint32(digitValue(self.chr))
			if digit >= base {
				// Not a valid digit
				goto skip
			}
			self.read()
		}
	} else {
		for self.chr != '}' && self.chr != -1 {
			digit := uint32(digitValue(self.chr))
			if digit >= base {
				// Not a valid digit
				goto skip
			}
			self.read()
		}
	}

	if length == 4 || length == 0 {
		self.write([]byte{
			'\\',
			'x',
			'{',
		})
		self.passString(valueOffset, self.chrOffset)
		if length != 0 {
			self.writeByte('}')
		}
	} else if length == 2 {
		self.passString(offset-1, valueOffset+2)
	} else {
		// Should never, ever get here...
		self.error(true, "re2: Illegal branch in scanEscape")
		return
	}

	return

skip:
	self.passString(offset, self.chrOffset)
}

func (self *_RegExp_parser) pass() {
	if self.passOffset == self.chrOffset {
		self.passOffset = self.offset
	} else {
		if self.passOffset != -1 {
			self.stopPassing()
		}
		if self.chr != -1 {
			self.goRegexp.WriteRune(self.chr)
		}
	}
	self.read()
}

func (self *_RegExp_parser) passString(start, end int) {
	if self.passOffset == start {
		self.passOffset = end
		return
	}
	if self.passOffset != -1 {
		self.stopPassing()
	}
	self.goRegexp.WriteString(self.str[start:end])
}

func (self *_RegExp_parser) error(fatal bool, msg string, msgValues ...interface{}) {
	if self.err != nil {
		return
	}
	e := regexpParseError{
		offset: self.offset,
		err:    fmt.Sprintf(msg, msgValues...),
	}
	if fatal {
		self.err = RegexpSyntaxError{e}
	} else {
		self.err = RegexpErrorIncompatible{e}
	}
	self.offset = self.length
	self.chr = -1
}
