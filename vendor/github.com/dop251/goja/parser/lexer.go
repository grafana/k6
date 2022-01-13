package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/rangetable"

	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

var (
	unicodeRangeIdNeg      = rangetable.Merge(unicode.Pattern_Syntax, unicode.Pattern_White_Space)
	unicodeRangeIdStartPos = rangetable.Merge(unicode.Letter, unicode.Nl, unicode.Other_ID_Start)
	unicodeRangeIdContPos  = rangetable.Merge(unicodeRangeIdStartPos, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
)

func isDecimalDigit(chr rune) bool {
	return '0' <= chr && chr <= '9'
}

func IsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	r, size := utf8.DecodeRuneInString(s)
	if !isIdentifierStart(r) {
		return false
	}
	for _, r := range s[size:] {
		if !isIdentifierPart(r) {
			return false
		}
	}
	return true
}

func digitValue(chr rune) int {
	switch {
	case '0' <= chr && chr <= '9':
		return int(chr - '0')
	case 'a' <= chr && chr <= 'f':
		return int(chr - 'a' + 10)
	case 'A' <= chr && chr <= 'F':
		return int(chr - 'A' + 10)
	}
	return 16 // Larger than any legal digit value
}

func isDigit(chr rune, base int) bool {
	return digitValue(chr) < base
}

func isIdStartUnicode(r rune) bool {
	return unicode.Is(unicodeRangeIdStartPos, r) && !unicode.Is(unicodeRangeIdNeg, r)
}

func isIdPartUnicode(r rune) bool {
	return unicode.Is(unicodeRangeIdContPos, r) && !unicode.Is(unicodeRangeIdNeg, r) || r == '\u200C' || r == '\u200D'
}

func isIdentifierStart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		chr >= utf8.RuneSelf && isIdStartUnicode(chr)
}

func isIdentifierPart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		'0' <= chr && chr <= '9' ||
		chr >= utf8.RuneSelf && isIdPartUnicode(chr)
}

func (self *_parser) scanIdentifier() (string, unistring.String, bool, string) {
	offset := self.chrOffset
	hasEscape := false
	isUnicode := false
	length := 0
	for isIdentifierPart(self.chr) {
		r := self.chr
		length++
		if r == '\\' {
			hasEscape = true
			distance := self.chrOffset - offset
			self.read()
			if self.chr != 'u' {
				return "", "", false, fmt.Sprintf("Invalid identifier escape character: %c (%s)", self.chr, string(self.chr))
			}
			var value rune
			if self._peek() == '{' {
				self.read()
				value = -1
				for value <= utf8.MaxRune {
					self.read()
					if self.chr == '}' {
						break
					}
					decimal, ok := hex2decimal(byte(self.chr))
					if !ok {
						return "", "", false, "Invalid Unicode escape sequence"
					}
					if value == -1 {
						value = decimal
					} else {
						value = value<<4 | decimal
					}
				}
				if value == -1 {
					return "", "", false, "Invalid Unicode escape sequence"
				}
			} else {
				for j := 0; j < 4; j++ {
					self.read()
					decimal, ok := hex2decimal(byte(self.chr))
					if !ok {
						return "", "", false, fmt.Sprintf("Invalid identifier escape character: %c (%s)", self.chr, string(self.chr))
					}
					value = value<<4 | decimal
				}
			}
			if value == '\\' {
				return "", "", false, fmt.Sprintf("Invalid identifier escape value: %c (%s)", value, string(value))
			} else if distance == 0 {
				if !isIdentifierStart(value) {
					return "", "", false, fmt.Sprintf("Invalid identifier escape value: %c (%s)", value, string(value))
				}
			} else if distance > 0 {
				if !isIdentifierPart(value) {
					return "", "", false, fmt.Sprintf("Invalid identifier escape value: %c (%s)", value, string(value))
				}
			}
			r = value
		}
		if r >= utf8.RuneSelf {
			isUnicode = true
			if r > 0xFFFF {
				length++
			}
		}
		self.read()
	}

	literal := self.str[offset:self.chrOffset]
	var parsed unistring.String
	if hasEscape || isUnicode {
		var err string
		// TODO strict
		parsed, err = parseStringLiteral(literal, length, isUnicode, false)
		if err != "" {
			return "", "", false, err
		}
	} else {
		parsed = unistring.String(literal)
	}

	return literal, parsed, hasEscape, ""
}

// 7.2
func isLineWhiteSpace(chr rune) bool {
	switch chr {
	case '\u0009', '\u000b', '\u000c', '\u0020', '\u00a0', '\ufeff':
		return true
	case '\u000a', '\u000d', '\u2028', '\u2029':
		return false
	case '\u0085':
		return false
	}
	return unicode.IsSpace(chr)
}

// 7.3
func isLineTerminator(chr rune) bool {
	switch chr {
	case '\u000a', '\u000d', '\u2028', '\u2029':
		return true
	}
	return false
}

type parserState struct {
	tok                                token.Token
	literal                            string
	parsedLiteral                      unistring.String
	implicitSemicolon, insertSemicolon bool
	chr                                rune
	chrOffset, offset                  int
	errorCount                         int
}

func (self *_parser) mark(state *parserState) *parserState {
	if state == nil {
		state = &parserState{}
	}
	state.tok, state.literal, state.parsedLiteral, state.implicitSemicolon, state.insertSemicolon, state.chr, state.chrOffset, state.offset =
		self.token, self.literal, self.parsedLiteral, self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset

	state.errorCount = len(self.errors)
	return state
}

func (self *_parser) restore(state *parserState) {
	self.token, self.literal, self.parsedLiteral, self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset =
		state.tok, state.literal, state.parsedLiteral, state.implicitSemicolon, state.insertSemicolon, state.chr, state.chrOffset, state.offset
	self.errors = self.errors[:state.errorCount]
}

func (self *_parser) peek() token.Token {
	implicitSemicolon, insertSemicolon, chr, chrOffset, offset := self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset
	tok, _, _, _ := self.scan()
	self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset = implicitSemicolon, insertSemicolon, chr, chrOffset, offset
	return tok
}

func (self *_parser) scan() (tkn token.Token, literal string, parsedLiteral unistring.String, idx file.Idx) {

	self.implicitSemicolon = false

	for {
		self.skipWhiteSpace()

		idx = self.idxOf(self.chrOffset)
		insertSemicolon := false

		switch chr := self.chr; {
		case isIdentifierStart(chr):
			var err string
			var hasEscape bool
			literal, parsedLiteral, hasEscape, err = self.scanIdentifier()
			if err != "" {
				tkn = token.ILLEGAL
				break
			}
			if len(parsedLiteral) > 1 {
				// Keywords are longer than 1 character, avoid lookup otherwise
				var strict bool
				tkn, strict = token.IsKeyword(string(parsedLiteral))
				if hasEscape {
					self.insertSemicolon = true
					if tkn != 0 && tkn != token.LET || parsedLiteral == "true" || parsedLiteral == "false" || parsedLiteral == "null" {
						tkn = token.KEYWORD
					} else {
						tkn = token.IDENTIFIER
					}
					return
				}
				switch tkn {

				case 0: // Not a keyword
					if parsedLiteral == "true" || parsedLiteral == "false" {
						self.insertSemicolon = true
						tkn = token.BOOLEAN
						return
					} else if parsedLiteral == "null" {
						self.insertSemicolon = true
						tkn = token.NULL
						return
					}

				case token.KEYWORD:
					if strict {
						// TODO If strict and in strict mode, then this is not a break
						break
					}
					return

				case
					token.THIS,
					token.BREAK,
					token.THROW, // A newline after a throw is not allowed, but we need to detect it
					token.RETURN,
					token.CONTINUE,
					token.DEBUGGER:
					self.insertSemicolon = true
					return

				default:
					return

				}
			}
			self.insertSemicolon = true
			tkn = token.IDENTIFIER
			return
		case '0' <= chr && chr <= '9':
			self.insertSemicolon = true
			tkn, literal = self.scanNumericLiteral(false)
			return
		default:
			self.read()
			switch chr {
			case -1:
				if self.insertSemicolon {
					self.insertSemicolon = false
					self.implicitSemicolon = true
				}
				tkn = token.EOF
			case '\r', '\n', '\u2028', '\u2029':
				self.insertSemicolon = false
				self.implicitSemicolon = true
				continue
			case ':':
				tkn = token.COLON
			case '.':
				if digitValue(self.chr) < 10 {
					insertSemicolon = true
					tkn, literal = self.scanNumericLiteral(true)
				} else {
					if self.chr == '.' {
						self.read()
						if self.chr == '.' {
							self.read()
							tkn = token.ELLIPSIS
						} else {
							tkn = token.ILLEGAL
						}
					} else {
						tkn = token.PERIOD
					}
				}
			case ',':
				tkn = token.COMMA
			case ';':
				tkn = token.SEMICOLON
			case '(':
				tkn = token.LEFT_PARENTHESIS
			case ')':
				tkn = token.RIGHT_PARENTHESIS
				insertSemicolon = true
			case '[':
				tkn = token.LEFT_BRACKET
			case ']':
				tkn = token.RIGHT_BRACKET
				insertSemicolon = true
			case '{':
				tkn = token.LEFT_BRACE
			case '}':
				tkn = token.RIGHT_BRACE
				insertSemicolon = true
			case '+':
				tkn = self.switch3(token.PLUS, token.ADD_ASSIGN, '+', token.INCREMENT)
				if tkn == token.INCREMENT {
					insertSemicolon = true
				}
			case '-':
				tkn = self.switch3(token.MINUS, token.SUBTRACT_ASSIGN, '-', token.DECREMENT)
				if tkn == token.DECREMENT {
					insertSemicolon = true
				}
			case '*':
				tkn = self.switch2(token.MULTIPLY, token.MULTIPLY_ASSIGN)
			case '/':
				if self.chr == '/' {
					self.skipSingleLineComment()
					continue
				} else if self.chr == '*' {
					self.skipMultiLineComment()
					continue
				} else {
					// Could be division, could be RegExp literal
					tkn = self.switch2(token.SLASH, token.QUOTIENT_ASSIGN)
					insertSemicolon = true
				}
			case '%':
				tkn = self.switch2(token.REMAINDER, token.REMAINDER_ASSIGN)
			case '^':
				tkn = self.switch2(token.EXCLUSIVE_OR, token.EXCLUSIVE_OR_ASSIGN)
			case '<':
				tkn = self.switch4(token.LESS, token.LESS_OR_EQUAL, '<', token.SHIFT_LEFT, token.SHIFT_LEFT_ASSIGN)
			case '>':
				tkn = self.switch6(token.GREATER, token.GREATER_OR_EQUAL, '>', token.SHIFT_RIGHT, token.SHIFT_RIGHT_ASSIGN, '>', token.UNSIGNED_SHIFT_RIGHT, token.UNSIGNED_SHIFT_RIGHT_ASSIGN)
			case '=':
				if self.chr == '>' {
					self.read()
					if self.implicitSemicolon {
						tkn = token.ILLEGAL
					} else {
						tkn = token.ARROW
					}
				} else {
					tkn = self.switch2(token.ASSIGN, token.EQUAL)
					if tkn == token.EQUAL && self.chr == '=' {
						self.read()
						tkn = token.STRICT_EQUAL
					}
				}
			case '!':
				tkn = self.switch2(token.NOT, token.NOT_EQUAL)
				if tkn == token.NOT_EQUAL && self.chr == '=' {
					self.read()
					tkn = token.STRICT_NOT_EQUAL
				}
			case '&':
				tkn = self.switch3(token.AND, token.AND_ASSIGN, '&', token.LOGICAL_AND)
			case '|':
				tkn = self.switch3(token.OR, token.OR_ASSIGN, '|', token.LOGICAL_OR)
			case '~':
				tkn = token.BITWISE_NOT
			case '?':
				if self.chr == '.' && !isDecimalDigit(self._peek()) {
					self.read()
					tkn = token.QUESTION_DOT
				} else {
					tkn = token.QUESTION_MARK
				}
			case '"', '\'':
				insertSemicolon = true
				tkn = token.STRING
				var err string
				literal, parsedLiteral, err = self.scanString(self.chrOffset-1, true)
				if err != "" {
					tkn = token.ILLEGAL
				}
			case '`':
				tkn = token.BACKTICK
			default:
				self.errorUnexpected(idx, chr)
				tkn = token.ILLEGAL
			}
		}
		self.insertSemicolon = insertSemicolon
		return
	}
}

func (self *_parser) switch2(tkn0, tkn1 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	return tkn0
}

func (self *_parser) switch3(tkn0, tkn1 token.Token, chr2 rune, tkn2 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		return tkn2
	}
	return tkn0
}

func (self *_parser) switch4(tkn0, tkn1 token.Token, chr2 rune, tkn2, tkn3 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		if self.chr == '=' {
			self.read()
			return tkn3
		}
		return tkn2
	}
	return tkn0
}

func (self *_parser) switch6(tkn0, tkn1 token.Token, chr2 rune, tkn2, tkn3 token.Token, chr3 rune, tkn4, tkn5 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		if self.chr == '=' {
			self.read()
			return tkn3
		}
		if self.chr == chr3 {
			self.read()
			if self.chr == '=' {
				self.read()
				return tkn5
			}
			return tkn4
		}
		return tkn2
	}
	return tkn0
}

func (self *_parser) _peek() rune {
	if self.offset < self.length {
		return rune(self.str[self.offset])
	}
	return -1
}

func (self *_parser) read() {
	if self.offset < self.length {
		self.chrOffset = self.offset
		chr, width := rune(self.str[self.offset]), 1
		if chr >= utf8.RuneSelf { // !ASCII
			chr, width = utf8.DecodeRuneInString(self.str[self.offset:])
			if chr == utf8.RuneError && width == 1 {
				self.error(self.chrOffset, "Invalid UTF-8 character")
			}
		}
		self.offset += width
		self.chr = chr
	} else {
		self.chrOffset = self.length
		self.chr = -1 // EOF
	}
}

func (self *_parser) skipSingleLineComment() {
	for self.chr != -1 {
		self.read()
		if isLineTerminator(self.chr) {
			return
		}
	}
}

func (self *_parser) skipMultiLineComment() {
	self.read()
	for self.chr >= 0 {
		chr := self.chr
		self.read()
		if chr == '*' && self.chr == '/' {
			self.read()
			return
		}
	}

	self.errorUnexpected(0, self.chr)
}

func (self *_parser) skipWhiteSpace() {
	for {
		switch self.chr {
		case ' ', '\t', '\f', '\v', '\u00a0', '\ufeff':
			self.read()
			continue
		case '\r':
			if self._peek() == '\n' {
				self.read()
			}
			fallthrough
		case '\u2028', '\u2029', '\n':
			if self.insertSemicolon {
				return
			}
			self.read()
			continue
		}
		if self.chr >= utf8.RuneSelf {
			if unicode.IsSpace(self.chr) {
				self.read()
				continue
			}
		}
		break
	}
}

func (self *_parser) scanMantissa(base int) {
	for digitValue(self.chr) < base {
		self.read()
	}
}

func (self *_parser) scanEscape(quote rune) (int, bool) {

	var length, base uint32
	chr := self.chr
	switch chr {
	case '0', '1', '2', '3', '4', '5', '6', '7':
		//    Octal:
		length, base = 3, 8
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', '"', '\'':
		self.read()
		return 1, false
	case '\r':
		self.read()
		if self.chr == '\n' {
			self.read()
			return 2, false
		}
		return 1, false
	case '\n':
		self.read()
		return 1, false
	case '\u2028', '\u2029':
		self.read()
		return 1, true
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
	default:
		self.read() // Always make progress
	}

	if base > 0 {
		var value uint32
		if length > 0 {
			for ; length > 0 && self.chr != quote && self.chr >= 0; length-- {
				digit := uint32(digitValue(self.chr))
				if digit >= base {
					break
				}
				value = value*base + digit
				self.read()
			}
		} else {
			for self.chr != quote && self.chr >= 0 && value < utf8.MaxRune {
				if self.chr == '}' {
					self.read()
					break
				}
				digit := uint32(digitValue(self.chr))
				if digit >= base {
					break
				}
				value = value*base + digit
				self.read()
			}
		}
		chr = rune(value)
	}
	if chr >= utf8.RuneSelf {
		if chr > 0xFFFF {
			return 2, true
		}
		return 1, true
	}
	return 1, false
}

func (self *_parser) scanString(offset int, parse bool) (literal string, parsed unistring.String, err string) {
	// " ' /
	quote := rune(self.str[offset])
	length := 0
	isUnicode := false
	for self.chr != quote {
		chr := self.chr
		if chr == '\n' || chr == '\r' || chr == '\u2028' || chr == '\u2029' || chr < 0 {
			goto newline
		}
		self.read()
		if chr == '\\' {
			if self.chr == '\n' || self.chr == '\r' || self.chr == '\u2028' || self.chr == '\u2029' || self.chr < 0 {
				if quote == '/' {
					goto newline
				}
				self.scanNewline()
			} else {
				l, u := self.scanEscape(quote)
				length += l
				if u {
					isUnicode = true
				}
			}
			continue
		} else if chr == '[' && quote == '/' {
			// Allow a slash (/) in a bracket character class ([...])
			// TODO Fix this, this is hacky...
			quote = -1
		} else if chr == ']' && quote == -1 {
			quote = '/'
		}
		if chr >= utf8.RuneSelf {
			isUnicode = true
			if chr > 0xFFFF {
				length++
			}
		}
		length++
	}

	// " ' /
	self.read()
	literal = self.str[offset:self.chrOffset]
	if parse {
		// TODO strict
		parsed, err = parseStringLiteral(literal[1:len(literal)-1], length, isUnicode, false)
	}
	return

newline:
	self.scanNewline()
	errStr := "String not terminated"
	if quote == '/' {
		errStr = "Invalid regular expression: missing /"
		self.error(self.idxOf(offset), errStr)
	}
	return "", "", errStr
}

func (self *_parser) scanNewline() {
	if self.chr == '\r' {
		self.read()
		if self.chr != '\n' {
			return
		}
	}
	self.read()
}

func (self *_parser) parseTemplateCharacters() (literal string, parsed unistring.String, finished bool, parseErr, err string) {
	offset := self.chrOffset
	var end int
	length := 0
	isUnicode := false
	hasCR := false
	for {
		chr := self.chr
		if chr < 0 {
			goto unterminated
		}
		self.read()
		if chr == '`' {
			finished = true
			end = self.chrOffset - 1
			break
		}
		if chr == '\\' {
			if self.chr == '\n' || self.chr == '\r' || self.chr == '\u2028' || self.chr == '\u2029' || self.chr < 0 {
				if self.chr == '\r' {
					hasCR = true
				}
				self.scanNewline()
			} else {
				if self.chr == '8' || self.chr == '9' {
					if parseErr == "" {
						parseErr = "\\8 and \\9 are not allowed in template strings."
					}
				}
				l, u := self.scanEscape('`')
				length += l
				if u {
					isUnicode = true
				}
			}
			continue
		}
		if chr == '$' && self.chr == '{' {
			self.read()
			end = self.chrOffset - 2
			break
		}
		if chr >= utf8.RuneSelf {
			isUnicode = true
			if chr > 0xFFFF {
				length++
			}
		} else if chr == '\r' {
			hasCR = true
			if self.chr == '\n' {
				length--
			}
		}
		length++
	}
	literal = self.str[offset:end]
	if hasCR {
		literal = normaliseCRLF(literal)
	}
	if parseErr == "" {
		parsed, parseErr = parseStringLiteral(literal, length, isUnicode, true)
	}
	self.insertSemicolon = true
	return
unterminated:
	err = err_UnexpectedEndOfInput
	finished = true
	return
}

func normaliseCRLF(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			buf.WriteByte('\n')
			if i < len(s)-1 && s[i+1] == '\n' {
				i++
			}
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.String()
}

func hex2decimal(chr byte) (value rune, ok bool) {
	{
		chr := rune(chr)
		switch {
		case '0' <= chr && chr <= '9':
			return chr - '0', true
		case 'a' <= chr && chr <= 'f':
			return chr - 'a' + 10, true
		case 'A' <= chr && chr <= 'F':
			return chr - 'A' + 10, true
		}
		return
	}
}

func parseNumberLiteral(literal string) (value interface{}, err error) {
	// TODO Is Uint okay? What about -MAX_UINT
	value, err = strconv.ParseInt(literal, 0, 64)
	if err == nil {
		return
	}

	parseIntErr := err // Save this first error, just in case

	value, err = strconv.ParseFloat(literal, 64)
	if err == nil {
		return
	} else if err.(*strconv.NumError).Err == strconv.ErrRange {
		// Infinity, etc.
		return value, nil
	}

	err = parseIntErr

	if err.(*strconv.NumError).Err == strconv.ErrRange {
		if len(literal) > 2 && literal[0] == '0' && (literal[1] == 'X' || literal[1] == 'x') {
			// Could just be a very large number (e.g. 0x8000000000000000)
			var value float64
			literal = literal[2:]
			for _, chr := range literal {
				digit := digitValue(chr)
				if digit >= 16 {
					goto error
				}
				value = value*16 + float64(digit)
			}
			return value, nil
		}
	}

error:
	return nil, errors.New("Illegal numeric literal")
}

func parseStringLiteral(literal string, length int, unicode, strict bool) (unistring.String, string) {
	var sb strings.Builder
	var chars []uint16
	if unicode {
		chars = make([]uint16, 1, length+1)
		chars[0] = unistring.BOM
	} else {
		sb.Grow(length)
	}
	str := literal
	for len(str) > 0 {
		switch chr := str[0]; {
		// We do not explicitly handle the case of the quote
		// value, which can be: " ' /
		// This assumes we're already passed a partially well-formed literal
		case chr >= utf8.RuneSelf:
			chr, size := utf8.DecodeRuneInString(str)
			if chr <= 0xFFFF {
				chars = append(chars, uint16(chr))
			} else {
				first, second := utf16.EncodeRune(chr)
				chars = append(chars, uint16(first), uint16(second))
			}
			str = str[size:]
			continue
		case chr != '\\':
			if unicode {
				chars = append(chars, uint16(chr))
			} else {
				sb.WriteByte(chr)
			}
			str = str[1:]
			continue
		}

		if len(str) <= 1 {
			panic("len(str) <= 1")
		}
		chr := str[1]
		var value rune
		if chr >= utf8.RuneSelf {
			str = str[1:]
			var size int
			value, size = utf8.DecodeRuneInString(str)
			str = str[size:] // \ + <character>
			if value == '\u2028' || value == '\u2029' {
				continue
			}
		} else {
			str = str[2:] // \<character>
			switch chr {
			case 'b':
				value = '\b'
			case 'f':
				value = '\f'
			case 'n':
				value = '\n'
			case 'r':
				value = '\r'
			case 't':
				value = '\t'
			case 'v':
				value = '\v'
			case 'x', 'u':
				size := 0
				switch chr {
				case 'x':
					size = 2
				case 'u':
					if str == "" || str[0] != '{' {
						size = 4
					}
				}
				if size > 0 {
					if len(str) < size {
						return "", fmt.Sprintf("invalid escape: \\%s: len(%q) != %d", string(chr), str, size)
					}
					for j := 0; j < size; j++ {
						decimal, ok := hex2decimal(str[j])
						if !ok {
							return "", fmt.Sprintf("invalid escape: \\%s: %q", string(chr), str[:size])
						}
						value = value<<4 | decimal
					}
				} else {
					str = str[1:]
					var val rune
					value = -1
					for ; size < len(str); size++ {
						if str[size] == '}' {
							if size == 0 {
								return "", fmt.Sprintf("invalid escape: \\%s", string(chr))
							}
							size++
							value = val
							break
						}
						decimal, ok := hex2decimal(str[size])
						if !ok {
							return "", fmt.Sprintf("invalid escape: \\%s: %q", string(chr), str[:size+1])
						}
						val = val<<4 | decimal
						if val > utf8.MaxRune {
							return "", fmt.Sprintf("undefined Unicode code-point: %q", str[:size+1])
						}
					}
					if value == -1 {
						return "", fmt.Sprintf("unterminated \\u{: %q", str)
					}
				}
				str = str[size:]
				if chr == 'x' {
					break
				}
				if value > utf8.MaxRune {
					panic("value > utf8.MaxRune")
				}
			case '0':
				if len(str) == 0 || '0' > str[0] || str[0] > '7' {
					value = 0
					break
				}
				fallthrough
			case '1', '2', '3', '4', '5', '6', '7':
				if strict {
					return "", "Octal escape sequences are not allowed in this context"
				}
				value = rune(chr) - '0'
				j := 0
				for ; j < 2; j++ {
					if len(str) < j+1 {
						break
					}
					chr := str[j]
					if '0' > chr || chr > '7' {
						break
					}
					decimal := rune(str[j]) - '0'
					value = (value << 3) | decimal
				}
				str = str[j:]
			case '\\':
				value = '\\'
			case '\'', '"':
				value = rune(chr)
			case '\r':
				if len(str) > 0 {
					if str[0] == '\n' {
						str = str[1:]
					}
				}
				fallthrough
			case '\n':
				continue
			default:
				value = rune(chr)
			}
		}
		if unicode {
			if value <= 0xFFFF {
				chars = append(chars, uint16(value))
			} else {
				first, second := utf16.EncodeRune(value)
				chars = append(chars, uint16(first), uint16(second))
			}
		} else {
			if value >= utf8.RuneSelf {
				return "", "Unexpected unicode character"
			}
			sb.WriteByte(byte(value))
		}
	}

	if unicode {
		if len(chars) != length+1 {
			panic(fmt.Errorf("unexpected unicode length while parsing '%s'", literal))
		}
		return unistring.FromUtf16(chars), ""
	}
	if sb.Len() != length {
		panic(fmt.Errorf("unexpected length while parsing '%s'", literal))
	}
	return unistring.String(sb.String()), ""
}

func (self *_parser) scanNumericLiteral(decimalPoint bool) (token.Token, string) {

	offset := self.chrOffset
	tkn := token.NUMBER

	if decimalPoint {
		offset--
		self.scanMantissa(10)
	} else {
		if self.chr == '0' {
			self.read()
			base := 0
			switch self.chr {
			case 'x', 'X':
				base = 16
			case 'o', 'O':
				base = 8
			case 'b', 'B':
				base = 2
			case '.', 'e', 'E':
				// no-op
			default:
				// legacy octal
				self.scanMantissa(8)
				goto end
			}
			if base > 0 {
				self.read()
				if !isDigit(self.chr, base) {
					return token.ILLEGAL, self.str[offset:self.chrOffset]
				}
				self.scanMantissa(base)
				goto end
			}
		} else {
			self.scanMantissa(10)
		}
		if self.chr == '.' {
			self.read()
			self.scanMantissa(10)
		}
	}

	if self.chr == 'e' || self.chr == 'E' {
		self.read()
		if self.chr == '-' || self.chr == '+' {
			self.read()
		}
		if isDecimalDigit(self.chr) {
			self.read()
			self.scanMantissa(10)
		} else {
			return token.ILLEGAL, self.str[offset:self.chrOffset]
		}
	}
end:
	if isIdentifierStart(self.chr) || isDecimalDigit(self.chr) {
		return token.ILLEGAL, self.str[offset:self.chrOffset]
	}

	return tkn, self.str[offset:self.chrOffset]
}
