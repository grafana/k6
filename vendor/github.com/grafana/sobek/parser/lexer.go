package parser

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/rangetable"

	"github.com/grafana/sobek/ast"
	"github.com/grafana/sobek/file"
	"github.com/grafana/sobek/token"
	"github.com/grafana/sobek/unistring"
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
	idx, prevTokenEnd                  file.Idx
	tok                                token.Token
	literal                            string
	parsedLiteral                      unistring.String
	sloppyLiteral                      bool
	numLiteralKind                     ast.NumberLiteralKind
	implicitSemicolon, insertSemicolon bool
	chr                                rune
	chrOffset, offset                  int
	errorCount                         int
}

func (self *_parser) mark(state *parserState) *parserState {
	if state == nil {
		state = &parserState{}
	}
	state.idx, state.prevTokenEnd, state.tok, state.literal, state.parsedLiteral, state.sloppyLiteral, state.numLiteralKind, state.implicitSemicolon, state.insertSemicolon, state.chr, state.chrOffset, state.offset =
		self.idx, self.prevTokenEnd, self.token, self.literal, self.parsedLiteral, self.sloppyLiteral, self.numLiteralKind, self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset

	state.errorCount = len(self.errors)
	return state
}

func (self *_parser) restore(state *parserState) {
	self.idx, self.prevTokenEnd, self.token, self.literal, self.parsedLiteral, self.sloppyLiteral, self.numLiteralKind, self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset =
		state.idx, state.prevTokenEnd, state.tok, state.literal, state.parsedLiteral, state.sloppyLiteral, state.numLiteralKind, state.implicitSemicolon, state.insertSemicolon, state.chr, state.chrOffset, state.offset
	self.errors = self.errors[:state.errorCount]
}

func (self *_parser) peek() token.Token {
	implicitSemicolon, insertSemicolon, chr, chrOffset, offset := self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset
	tok, _, _, _, _, _ := self.scan()
	self.implicitSemicolon, self.insertSemicolon, self.chr, self.chrOffset, self.offset = implicitSemicolon, insertSemicolon, chr, chrOffset, offset
	return tok
}

func (self *_parser) scan() (tkn token.Token, literal string, parsedLiteral unistring.String, sloppy bool, numLiteralKind ast.NumberLiteralKind, idx file.Idx) {

	self.implicitSemicolon = false
	start := self.chrOffset

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
					if tkn == 0 || self.isBindingId(tkn) {
						tkn = token.IDENTIFIER
					} else {
						tkn = token.ESCAPED_RESERVED_WORD
					}
					return
				}
				switch tkn {
				case 0: // Not a keyword
					// no-op
				case token.KEYWORD:
					if strict {
						// TODO If strict and in strict mode, then this is not a break
						break
					}
					return

				case
					token.BOOLEAN,
					token.NULL,
					token.THIS,
					token.BREAK,
					token.THROW, // A newline after a throw is not allowed, but we need to detect it
					token.YIELD,
					token.RETURN,
					token.CONTINUE,
					token.DEBUGGER:
					self.insertSemicolon = true
					return

				case token.ASYNC:
					// async only has special meaning if not followed by a LineTerminator
					if self.skipWhiteSpaceCheckLineTerminator() {
						self.insertSemicolon = true
						tkn = token.IDENTIFIER
					}
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
			tkn, literal, numLiteralKind = self.scanNumericLiteral(false)
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
					tkn, literal, numLiteralKind = self.scanNumericLiteral(true)
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
				if !self.opts.module && self.chr == '-' && self._peek() == '>' && (start == 0 || strings.ContainsAny(self.str[start:self.chrOffset], "\r\n\u2028\u2029")) {
					self.skipSingleLineComment()
					continue
				}
				tkn = self.switch3(token.MINUS, token.SUBTRACT_ASSIGN, '-', token.DECREMENT)
				if tkn == token.DECREMENT {
					insertSemicolon = true
				}
			case '*':
				if self.chr == '*' {
					self.read()
					tkn = self.switch2(token.EXPONENT, token.EXPONENT_ASSIGN)
				} else {
					tkn = self.switch2(token.MULTIPLY, token.MULTIPLY_ASSIGN)
				}
			case '/':
				if self.chr == '/' {
					self.skipSingleLineComment()
					continue
				} else if self.chr == '*' {
					if self.skipMultiLineComment() {
						self.insertSemicolon = false
						self.implicitSemicolon = true
					}
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
				if !self.opts.module && strings.HasPrefix(self.str[self.chrOffset:], "!--") {
					self.skipSingleLineComment()
					continue
				}
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
				tkn = self.switch4(token.AND, token.AND_ASSIGN, '&', token.LOGICAL_AND, token.LOGICAL_AND_ASSIGN)
			case '|':
				tkn = self.switch4(token.OR, token.OR_ASSIGN, '|', token.LOGICAL_OR, token.LOGICAL_OR_ASSIGN)
			case '~':
				tkn = token.BITWISE_NOT
			case '?':
				if self.chr == '.' && !isDecimalDigit(self._peek()) {
					self.read()
					tkn = token.QUESTION_DOT
				} else if self.chr == '?' {
					self.read()
					if self.chr == '=' {
						self.read()
						tkn = token.COALESCE_ASSIGN
					} else {
						tkn = token.COALESCE
					}
				} else {
					tkn = token.QUESTION_MARK
				}
			case '"', '\'':
				insertSemicolon = true
				tkn = token.STRING
				var err string
				literal, parsedLiteral, sloppy, err = self.scanString(self.chrOffset-1, true)
				if err != "" {
					tkn = token.ILLEGAL
				}
			case '`':
				tkn = token.BACKTICK
			case '#':
				if self.chrOffset == 1 && self.chr == '!' {
					self.skipSingleLineComment()
					continue
				}

				var err string
				literal, parsedLiteral, _, err = self.scanIdentifier()
				if err != "" || literal == "" {
					tkn = token.ILLEGAL
					break
				}
				self.insertSemicolon = true
				tkn = token.PRIVATE_IDENTIFIER
				return
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

func (self *_parser) skipMultiLineComment() (hasLineTerminator bool) {
	self.read()
	for self.chr >= 0 {
		chr := self.chr
		if chr == '\r' || chr == '\n' || chr == '\u2028' || chr == '\u2029' {
			hasLineTerminator = true
			break
		}
		self.read()
		if chr == '*' && self.chr == '/' {
			self.read()
			return
		}
	}
	for self.chr >= 0 {
		chr := self.chr
		self.read()
		if chr == '*' && self.chr == '/' {
			self.read()
			return
		}
	}

	self.errorUnexpected(0, self.chr)
	return
}

func (self *_parser) skipWhiteSpaceCheckLineTerminator() bool {
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
			return true
		}
		if self.chr >= utf8.RuneSelf {
			if unicode.IsSpace(self.chr) {
				self.read()
				continue
			}
		}
		break
	}
	return false
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

func (self *_parser) scanMantissa(base int, allowSeparator bool) {
	for digitValue(self.chr) < base || (allowSeparator && self.chr == '_') {
		afterUnderscore := self.chr == '_'
		self.read()
		if afterUnderscore && !isDigit(self.chr, base) {
			self.error(self.chrOffset, "Only one underscore is allowed as numeric separator")
		}
	}
}

func (self *_parser) scanEscape(quote rune) (int, bool) {

	var length, base uint32
	chr := self.chr
	switch chr {
	case '0', '1', '2', '3':
		//    Octal:
		length, base = 3, 8
	case '4', '5', '6', '7':
		//    Octal, at most two digits:
		length, base = 2, 8
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
			for self.chr != quote && self.chr >= 0 && value <= utf8.MaxRune {
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

func (self *_parser) scanString(offset int, parse bool) (literal string, parsed unistring.String, sloppy bool, err string) {
	// " ' /
	quote := rune(self.str[offset])
	length := 0
	isUnicode := false
	for self.chr != quote {
		chr := self.chr
		if chr == '\n' || chr == '\r' || chr < 0 {
			goto newline
		}
		if quote == '/' && (self.chr == '\u2028' || self.chr == '\u2029') {
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
				// Legacy octal (\0 followed by a digit, \1 to \7) and \8, \9 escapes are not allowed in strict mode
				if isDecimalDigit(self.chr) && (self.chr != '0' || isDecimalDigit(self._peek())) {
					sloppy = true
				}
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
	return "", "", false, errStr
}

func (self *_parser) scanNewline() {
	if self.chr == '\u2028' || self.chr == '\u2029' {
		self.read()
		return
	}
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

// roundToFloat64 returns the float64 nearest to (mant * 2^exp), rounding
// ties to even. sticky reports whether any nonzero bits were discarded
// below mant's least significant bit.
func roundToFloat64(mant uint64, exp int, sticky bool) float64 {
	if mant == 0 {
		return 0
	}
	n := bits.Len64(mant)
	if n <= 53 {
		// Exact. sticky is necessarily false here: it is only set once
		// the accumulator is full, which implies n >= 61.
		// This should never happen as parsePow2Radix is only called for values that
		// overflow uint64. Left for completeness.
		return math.Ldexp(float64(mant), exp)
	}

	shift := uint(n - 53)
	m := mant >> shift
	rem := mant & (1<<shift - 1)
	half := uint64(1) << (shift - 1)

	// Round to nearest; on an exact tie, the sticky bit means the true
	// value is above the midpoint, otherwise round to even.
	if rem > half || (rem == half && (sticky || m&1 == 1)) {
		m++ // may reach 2^53, which float64 still represents exactly
	}
	return math.Ldexp(float64(m), exp+int(shift))
}

func parsePow2Radix(digits string, bitsPerDigit uint) float64 {
	var mant uint64
	var exp int
	sticky := false
	bitsUsed := 0

	// Skip leading zeros (including any separators between them) so they don't
	// consume mantissa capacity. This also guarantees that the first digit
	// entering the accumulator is significant, so whenever a digit is dropped
	// (mant full, bitsUsed > 64-bitsPerDigit) mant's bit length is > 53 and
	// roundToFloat64's sticky argument is honoured.
	i := 0
	for i < len(digits) && (digits[i] == '0' || digits[i] == '_') {
		i++
	}

	for ; i < len(digits); i++ {
		c := digits[i]
		if c == '_' {
			continue
		}
		d := uint64(digitValue(rune(c)))
		if bitsUsed+int(bitsPerDigit) <= 64 {
			mant = mant<<bitsPerDigit | d
			bitsUsed += int(bitsPerDigit)
			continue
		}
		exp += int(bitsPerDigit)
		if d != 0 {
			sticky = true
		}
	}
	return roundToFloat64(mant, exp, sticky)
}

// Assumes pre-scanned and pre-validated literals (e.g. double group separators already rejected)
// and that kind matches the lexical form of the literal as determined by the scanner.
func parseNumber(literal string, kind ast.NumberLiteralKind) (value any, err error) {
	value, err = strconv.ParseInt(literal, 0, 64)
	if err == nil {
		return
	}

	parseIntErr := err

	// Doesn't fit into int64, but may fit into uint64 (e.g. 0x8000000000000000).
	// Converting it to float64 yields the correctly rounded value of the literal.
	if u, err := strconv.ParseUint(literal, 0, 64); err == nil {
		return float64(u), nil
	}

	base := 10
	switch kind {
	case ast.NumberLiteralLegacyOctal:
		// Legacy octal integer that doesn't fit into int64 (e.g. 07777777777777777777777).
		// It must not be parsed as a decimal number, so skip ParseFloat and handle it
		// via the exact base-8 path below.
		base = 8
	case ast.NumberLiteralPlain:
		if len(literal) > 2 && literal[0] == '0' {
			switch literal[1] {
			case 'x', 'X':
				literal, base = literal[2:], 16
			case 'b', 'B':
				literal, base = literal[2:], 2
			case 'o', 'O':
				literal, base = literal[2:], 8
			}
		}
	}

	if base == 10 {
		// ParseFloat cannot handle radix-prefixed literals, so for those (as well as
		// for legacy octal handled above) go straight to the exact path below.
		value, err = strconv.ParseFloat(literal, 64)
		if err == nil {
			return
		} else if errors.Is(err, strconv.ErrRange) {
			// Infinity, etc.
			return value, nil
		}
	} else if errors.Is(parseIntErr, strconv.ErrRange) {
		// The value is too large even for uint64; parse it exactly and round it
		// to the nearest float64 (round-to-nearest-even) as required for numeric
		// literals. The Horner scheme in float64 would accumulate rounding errors.
		var bitsPerDigit uint
		switch base {
		case 16:
			bitsPerDigit = 4
		case 8:
			bitsPerDigit = 3
		case 2:
			bitsPerDigit = 1
		}
		return parsePow2Radix(literal, bitsPerDigit), nil
	}

	return nil, errors.New("Illegal numeric literal")
}

func parseBigInt(literal string) (*big.Int, error) {
	bigInt := new(big.Int)
	if len(literal) >= 2 && literal[0] == '0' {
		switch literal[1] {
		case 'x', 'X', 'b', 'B', 'o', 'O':
		default:
			return nil, errors.New("Illegal numeric literal")
		}
	}

	_, ok := bigInt.SetString(literal, 0)
	if !ok {
		return nil, errors.New("Illegal numeric literal")
	}
	return bigInt, nil
}

func parseNumberLiteral(literal string, kind ast.NumberLiteralKind) (value any, err error) {
	if len(literal) > 1 && literal[len(literal)-1] == 'n' {
		return parseBigInt(literal[:len(literal)-1])
	}
	return parseNumber(literal, kind)
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
				maxDigits := 2
				if chr >= '4' {
					// \4 to \7 can only be followed by one more octal digit
					maxDigits = 1
				}
				j := 0
				for ; j < maxDigits; j++ {
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

func (self *_parser) scanNumericLiteral(decimalPoint bool) (token.Token, string, ast.NumberLiteralKind) {

	offset := self.chrOffset
	tkn := token.NUMBER
	kind := ast.NumberLiteralPlain

	if decimalPoint {
		offset--
		self.scanMantissa(10, true)
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
				self.scanMantissa(8, false)
				if !isDecimalDigit(self.chr) {
					if self.chrOffset > offset+1 {
						// At least one octal digit after the leading '0'
						kind = ast.NumberLiteralLegacyOctal
					}
					goto end
				}
				// A decimal digit (necessarily 8 or 9) follows the octal digits:
				// a decimal number with a leading zero, e.g. 089
				kind = ast.NumberLiteralNonOctalDecimal
				self.scanMantissa(10, false)
			}
			if base > 0 {
				self.read()
				if !isDigit(self.chr, base) {
					return token.ILLEGAL, self.str[offset:self.chrOffset], kind
				}
				self.scanMantissa(base, true)
				goto end
			}
		} else {
			self.scanMantissa(10, true)
		}
		if self.chr == '.' {
			self.read()
			self.scanMantissa(10, true)
		}
	}

	if self.chr == 'e' || self.chr == 'E' {
		self.read()
		if self.chr == '-' || self.chr == '+' {
			self.read()
		}
		if isDecimalDigit(self.chr) {
			self.read()
			self.scanMantissa(10, true)
		} else {
			return token.ILLEGAL, self.str[offset:self.chrOffset], kind
		}
	}
end:
	if self.chr == 'n' || self.chr == 'N' {
		self.read()
		return tkn, self.str[offset:self.chrOffset], kind
	}
	if isIdentifierStart(self.chr) || isDecimalDigit(self.chr) {
		return token.ILLEGAL, self.str[offset:self.chrOffset], kind
	}

	return tkn, self.str[offset:self.chrOffset], kind
}
