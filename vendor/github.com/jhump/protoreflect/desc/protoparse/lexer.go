package protoparse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type runeReader struct {
	rr     *bufio.Reader
	unread []rune
	err    error
}

func (rr *runeReader) readRune() (r rune, size int, err error) {
	if rr.err != nil {
		return 0, 0, rr.err
	}
	if len(rr.unread) > 0 {
		r := rr.unread[len(rr.unread)-1]
		rr.unread = rr.unread[:len(rr.unread)-1]
		return r, utf8.RuneLen(r), nil
	}
	r, sz, err := rr.rr.ReadRune()
	if err != nil {
		rr.err = err
	}
	return r, sz, err
}

func (rr *runeReader) unreadRune(r rune) {
	rr.unread = append(rr.unread, r)
}

func lexError(l protoLexer, pos *SourcePos, err string) {
	pl := l.(*protoLex)
	_ = pl.errs.handleErrorWithPos(pos, err)
}

type protoLex struct {
	filename string
	input    *runeReader
	errs     *errorHandler
	res      *fileNode

	lineNo int
	colNo  int
	offset int

	prevSym terminalNode

	prevLineNo int
	prevColNo  int
	prevOffset int
	comments   []comment
}

func newLexer(in io.Reader, filename string, errs *errorHandler) *protoLex {
	return &protoLex{
		input:    &runeReader{rr: bufio.NewReader(in)},
		filename: filename,
		errs:     errs,
	}
}

var keywords = map[string]int{
	"syntax":     _SYNTAX,
	"import":     _IMPORT,
	"weak":       _WEAK,
	"public":     _PUBLIC,
	"package":    _PACKAGE,
	"option":     _OPTION,
	"true":       _TRUE,
	"false":      _FALSE,
	"inf":        _INF,
	"nan":        _NAN,
	"repeated":   _REPEATED,
	"optional":   _OPTIONAL,
	"required":   _REQUIRED,
	"double":     _DOUBLE,
	"float":      _FLOAT,
	"int32":      _INT32,
	"int64":      _INT64,
	"uint32":     _UINT32,
	"uint64":     _UINT64,
	"sint32":     _SINT32,
	"sint64":     _SINT64,
	"fixed32":    _FIXED32,
	"fixed64":    _FIXED64,
	"sfixed32":   _SFIXED32,
	"sfixed64":   _SFIXED64,
	"bool":       _BOOL,
	"string":     _STRING,
	"bytes":      _BYTES,
	"group":      _GROUP,
	"oneof":      _ONEOF,
	"map":        _MAP,
	"extensions": _EXTENSIONS,
	"to":         _TO,
	"max":        _MAX,
	"reserved":   _RESERVED,
	"enum":       _ENUM,
	"message":    _MESSAGE,
	"extend":     _EXTEND,
	"service":    _SERVICE,
	"rpc":        _RPC,
	"stream":     _STREAM,
	"returns":    _RETURNS,
}

func (l *protoLex) cur() SourcePos {
	return SourcePos{
		Filename: l.filename,
		Offset:   l.offset,
		Line:     l.lineNo + 1,
		Col:      l.colNo + 1,
	}
}

func (l *protoLex) adjustPos(consumedChars ...rune) {
	for _, c := range consumedChars {
		switch c {
		case '\n':
			// new line, back to first column
			l.colNo = 0
			l.lineNo++
		case '\r':
			// no adjustment
		case '\t':
			// advance to next tab stop
			mod := l.colNo % 8
			l.colNo += 8 - mod
		default:
			l.colNo++
		}
	}
}

func (l *protoLex) prev() *SourcePos {
	if l.prevSym == nil {
		return &SourcePos{
			Filename: l.filename,
			Offset:   0,
			Line:     1,
			Col:      1,
		}
	}
	return l.prevSym.start()
}

func (l *protoLex) Lex(lval *protoSymType) int {
	if l.errs.err != nil {
		// if error reporter already returned non-nil error,
		// we can skip the rest of the input
		return 0
	}

	l.prevLineNo = l.lineNo
	l.prevColNo = l.colNo
	l.prevOffset = l.offset
	l.comments = nil

	for {
		c, n, err := l.input.readRune()
		if err == io.EOF {
			// we're not actually returning a rune, but this will associate
			// accumulated comments as a trailing comment on last symbol
			// (if appropriate)
			l.setRune(lval)
			return 0
		} else if err != nil {
			// we don't call setError because we don't want it wrapped
			// with a source position because it's I/O, not syntax
			lval.err = err
			_ = l.errs.handleError(err)
			return _ERROR
		}

		l.prevLineNo = l.lineNo
		l.prevColNo = l.colNo
		l.prevOffset = l.offset

		l.offset += n
		l.adjustPos(c)
		if strings.ContainsRune("\n\r\t ", c) {
			continue
		}

		if c == '.' {
			// decimal literals could start with a dot
			cn, _, err := l.input.readRune()
			if err != nil {
				l.setDot(lval)
				return int(c)
			}
			if cn >= '0' && cn <= '9' {
				l.adjustPos(cn)
				token := []rune{c, cn}
				token = l.readNumber(token, false, true)
				f, err := strconv.ParseFloat(string(token), 64)
				if err != nil {
					l.setError(lval, err)
					return _ERROR
				}
				l.setFloat(lval, f)
				return _FLOAT_LIT
			}
			l.input.unreadRune(cn)
			l.setDot(lval)
			return int(c)
		}

		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			// identifier
			token := []rune{c}
			token = l.readIdentifier(token)
			str := string(token)
			if t, ok := keywords[str]; ok {
				l.setIdent(lval, str)
				return t
			}
			l.setIdent(lval, str)
			return _NAME
		}

		if c >= '0' && c <= '9' {
			// integer or float literal
			if c == '0' {
				cn, _, err := l.input.readRune()
				if err != nil {
					l.setInt(lval, 0)
					return _INT_LIT
				}
				if cn == 'x' || cn == 'X' {
					cnn, _, err := l.input.readRune()
					if err != nil {
						l.input.unreadRune(cn)
						l.setInt(lval, 0)
						return _INT_LIT
					}
					if (cnn >= '0' && cnn <= '9') || (cnn >= 'a' && cnn <= 'f') || (cnn >= 'A' && cnn <= 'F') {
						// hexadecimal!
						l.adjustPos(cn, cnn)
						token := []rune{cnn}
						token = l.readHexNumber(token)
						ui, err := strconv.ParseUint(string(token), 16, 64)
						if err != nil {
							l.setError(lval, err)
							return _ERROR
						}
						l.setInt(lval, ui)
						return _INT_LIT
					}
					l.input.unreadRune(cnn)
					l.input.unreadRune(cn)
					l.setInt(lval, 0)
					return _INT_LIT
				} else {
					l.input.unreadRune(cn)
				}
			}
			token := []rune{c}
			token = l.readNumber(token, true, true)
			numstr := string(token)
			if strings.Contains(numstr, ".") || strings.Contains(numstr, "e") || strings.Contains(numstr, "E") {
				// floating point!
				f, err := strconv.ParseFloat(numstr, 64)
				if err != nil {
					l.setError(lval, err)
					return _ERROR
				}
				l.setFloat(lval, f)
				return _FLOAT_LIT
			}
			// integer! (decimal or octal)
			ui, err := strconv.ParseUint(numstr, 0, 64)
			if err != nil {
				if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
					// if it's too big to be an int, parse it as a float
					var f float64
					f, err = strconv.ParseFloat(numstr, 64)
					if err == nil {
						l.setFloat(lval, f)
						return _FLOAT_LIT
					}
				}
				l.setError(lval, err)
				return _ERROR
			}
			l.setInt(lval, ui)
			return _INT_LIT
		}

		if c == '\'' || c == '"' {
			// string literal
			str, err := l.readStringLiteral(c)
			if err != nil {
				l.setError(lval, err)
				return _ERROR
			}
			l.setString(lval, str)
			return _STRING_LIT
		}

		if c == '/' {
			// comment
			cn, _, err := l.input.readRune()
			if err != nil {
				l.setRune(lval)
				return int(c)
			}
			if cn == '/' {
				l.adjustPos(cn)
				hitNewline, txt := l.skipToEndOfLineComment()
				commentPos := l.posRange()
				commentPos.end.Col++
				if hitNewline {
					// we don't do this inside of skipToEndOfLineComment
					// because we want to know the length of previous
					// line for calculation above
					l.adjustPos('\n')
				}
				l.comments = append(l.comments, comment{posRange: commentPos, text: txt})
				continue
			}
			if cn == '*' {
				l.adjustPos(cn)
				if txt, ok := l.skipToEndOfBlockComment(); !ok {
					l.setError(lval, errors.New("block comment never terminates, unexpected EOF"))
					return _ERROR
				} else {
					l.comments = append(l.comments, comment{posRange: l.posRange(), text: txt})
				}
				continue
			}
			l.input.unreadRune(cn)
		}

		l.setRune(lval)
		return int(c)
	}
}

func (l *protoLex) posRange() posRange {
	return posRange{
		start: SourcePos{
			Filename: l.filename,
			Offset:   l.prevOffset,
			Line:     l.prevLineNo + 1,
			Col:      l.prevColNo + 1,
		},
		end: l.cur(),
	}
}

func (l *protoLex) newBasicNode() basicNode {
	return basicNode{
		posRange: l.posRange(),
		leading:  l.comments,
	}
}

func (l *protoLex) setPrev(n terminalNode, isDot bool) {
	nStart := n.start().Line
	if _, ok := n.(*basicNode); ok {
		// This is really gross, but there are many cases where we don't want
		// to attribute comments to punctuation (like commas, equals, semicolons)
		// and would instead prefer to attribute comments to a more meaningful
		// element in the AST.
		//
		// So if it's a simple node OTHER THAN PERIOD (since that is not just
		// punctuation but typically part of a qualified identifier), don't
		// attribute comments to it. We do that with this TOTAL HACK: adjusting
		// the start line makes leading comments appear detached so logic below
		// will naturally associated trailing comment to previous symbol
		if !isDot {
			nStart += 2
		}
	}
	if l.prevSym != nil && len(n.leadingComments()) > 0 && l.prevSym.end().Line < nStart {
		// we may need to re-attribute the first comment to
		// instead be previous node's trailing comment
		prevEnd := l.prevSym.end().Line
		comments := n.leadingComments()
		c := comments[0]
		commentStart := c.start.Line
		if commentStart == prevEnd {
			// comment is on same line as previous symbol
			n.popLeadingComment()
			l.prevSym.pushTrailingComment(c)
		} else if commentStart == prevEnd+1 {
			// comment is right after previous symbol; see if it is detached
			// and if so re-attribute
			singleLineStyle := strings.HasPrefix(c.text, "//")
			line := c.end.Line
			groupEnd := -1
			for i := 1; i < len(comments); i++ {
				c := comments[i]
				newGroup := false
				if !singleLineStyle || c.start.Line > line+1 {
					// we've found a gap between comments, which means the
					// previous comments were detached
					newGroup = true
				} else {
					line = c.end.Line
					singleLineStyle = strings.HasPrefix(comments[i].text, "//")
					if !singleLineStyle {
						// we've found a switch from // comments to /*
						// consider that a new group which means the
						// previous comments were detached
						newGroup = true
					}
				}
				if newGroup {
					groupEnd = i
					break
				}
			}

			if groupEnd == -1 {
				// just one group of comments; we'll mark it as a trailing
				// comment if it immediately follows previous symbol and is
				// detached from current symbol
				c1 := comments[0]
				c2 := comments[len(comments)-1]
				if c1.start.Line <= prevEnd+1 && c2.end.Line < nStart-1 {
					groupEnd = len(comments)
				}
			}

			for i := 0; i < groupEnd; i++ {
				l.prevSym.pushTrailingComment(n.popLeadingComment())
			}
		}
	}

	l.prevSym = n
}

func (l *protoLex) setString(lval *protoSymType, val string) {
	lval.s = &stringLiteralNode{basicNode: l.newBasicNode(), val: val}
	l.setPrev(lval.s, false)
}

func (l *protoLex) setIdent(lval *protoSymType, val string) {
	lval.id = &identNode{basicNode: l.newBasicNode(), val: val}
	l.setPrev(lval.id, false)
}

func (l *protoLex) setInt(lval *protoSymType, val uint64) {
	lval.i = &intLiteralNode{basicNode: l.newBasicNode(), val: val}
	l.setPrev(lval.i, false)
}

func (l *protoLex) setFloat(lval *protoSymType, val float64) {
	lval.f = &floatLiteralNode{basicNode: l.newBasicNode(), val: val}
	l.setPrev(lval.f, false)
}

func (l *protoLex) setRune(lval *protoSymType) {
	b := l.newBasicNode()
	lval.b = &b
	l.setPrev(lval.b, false)
}

func (l *protoLex) setDot(lval *protoSymType) {
	b := l.newBasicNode()
	lval.b = &b
	l.setPrev(lval.b, true)
}

func (l *protoLex) setError(lval *protoSymType, err error) {
	lval.err = l.addSourceError(err)
}

func (l *protoLex) readNumber(sofar []rune, allowDot bool, allowExp bool) []rune {
	token := sofar
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			break
		}
		if c == '.' {
			if !allowDot {
				l.input.unreadRune(c)
				break
			}
			allowDot = false
		} else if c == 'e' || c == 'E' {
			if !allowExp {
				l.input.unreadRune(c)
				break
			}
			allowExp = false
			cn, _, err := l.input.readRune()
			if err != nil {
				l.input.unreadRune(c)
				break
			}
			if cn == '-' || cn == '+' {
				cnn, _, err := l.input.readRune()
				if err != nil {
					l.input.unreadRune(cn)
					l.input.unreadRune(c)
					break
				}
				if cnn < '0' || cnn > '9' {
					l.input.unreadRune(cnn)
					l.input.unreadRune(cn)
					l.input.unreadRune(c)
					break
				}
				l.adjustPos(c)
				token = append(token, c)
				c, cn = cn, cnn
			} else if cn < '0' || cn > '9' {
				l.input.unreadRune(cn)
				l.input.unreadRune(c)
				break
			}
			l.adjustPos(c)
			token = append(token, c)
			c = cn
		} else if c < '0' || c > '9' {
			l.input.unreadRune(c)
			break
		}
		l.adjustPos(c)
		token = append(token, c)
	}
	return token
}

func (l *protoLex) readHexNumber(sofar []rune) []rune {
	token := sofar
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			break
		}
		if (c < 'a' || c > 'f') && (c < 'A' || c > 'F') && (c < '0' || c > '9') {
			l.input.unreadRune(c)
			break
		}
		l.adjustPos(c)
		token = append(token, c)
	}
	return token
}

func (l *protoLex) readIdentifier(sofar []rune) []rune {
	token := sofar
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			break
		}
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			l.input.unreadRune(c)
			break
		}
		l.adjustPos(c)
		token = append(token, c)
	}
	return token
}

func (l *protoLex) readStringLiteral(quote rune) (string, error) {
	var buf bytes.Buffer
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return "", err
		}
		if c == '\n' {
			return "", errors.New("encountered end-of-line before end of string literal")
		}
		l.adjustPos(c)
		if c == quote {
			break
		}
		if c == 0 {
			return "", errors.New("null character ('\\0') not allowed in string literal")
		}
		if c == '\\' {
			// escape sequence
			c, _, err = l.input.readRune()
			if err != nil {
				return "", err
			}
			l.adjustPos(c)
			if c == 'x' || c == 'X' {
				// hex escape
				c, _, err := l.input.readRune()
				if err != nil {
					return "", err
				}
				l.adjustPos(c)
				c2, _, err := l.input.readRune()
				if err != nil {
					return "", err
				}
				var hex string
				if (c2 < '0' || c2 > '9') && (c2 < 'a' || c2 > 'f') && (c2 < 'A' || c2 > 'F') {
					l.input.unreadRune(c2)
					hex = string(c)
				} else {
					l.adjustPos(c2)
					hex = string([]rune{c, c2})
				}
				i, err := strconv.ParseInt(hex, 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid hex escape: \\x%q", hex)
				}
				buf.WriteByte(byte(i))

			} else if c >= '0' && c <= '7' {
				// octal escape
				c2, _, err := l.input.readRune()
				if err != nil {
					return "", err
				}
				var octal string
				if c2 < '0' || c2 > '7' {
					l.input.unreadRune(c2)
					octal = string(c)
				} else {
					l.adjustPos(c2)
					c3, _, err := l.input.readRune()
					if err != nil {
						return "", err
					}
					if c3 < '0' || c3 > '7' {
						l.input.unreadRune(c3)
						octal = string([]rune{c, c2})
					} else {
						l.adjustPos(c3)
						octal = string([]rune{c, c2, c3})
					}
				}
				i, err := strconv.ParseInt(octal, 8, 32)
				if err != nil {
					return "", fmt.Errorf("invalid octal escape: \\%q", octal)
				}
				if i > 0xff {
					return "", fmt.Errorf("octal escape is out range, must be between 0 and 377: \\%q", octal)
				}
				buf.WriteByte(byte(i))

			} else if c == 'u' {
				// short unicode escape
				u := make([]rune, 4)
				for i := range u {
					c, _, err := l.input.readRune()
					if err != nil {
						return "", err
					}
					l.adjustPos(c)
					u[i] = c
				}
				i, err := strconv.ParseInt(string(u), 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid unicode escape: \\u%q", string(u))
				}
				buf.WriteRune(rune(i))

			} else if c == 'U' {
				// long unicode escape
				u := make([]rune, 8)
				for i := range u {
					c, _, err := l.input.readRune()
					if err != nil {
						return "", err
					}
					l.adjustPos(c)
					u[i] = c
				}
				i, err := strconv.ParseInt(string(u), 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid unicode escape: \\U%q", string(u))
				}
				if i > 0x10ffff || i < 0 {
					return "", fmt.Errorf("unicode escape is out of range, must be between 0 and 0x10ffff: \\U%q", string(u))
				}
				buf.WriteRune(rune(i))

			} else if c == 'a' {
				buf.WriteByte('\a')
			} else if c == 'b' {
				buf.WriteByte('\b')
			} else if c == 'f' {
				buf.WriteByte('\f')
			} else if c == 'n' {
				buf.WriteByte('\n')
			} else if c == 'r' {
				buf.WriteByte('\r')
			} else if c == 't' {
				buf.WriteByte('\t')
			} else if c == 'v' {
				buf.WriteByte('\v')
			} else if c == '\\' {
				buf.WriteByte('\\')
			} else if c == '\'' {
				buf.WriteByte('\'')
			} else if c == '"' {
				buf.WriteByte('"')
			} else if c == '?' {
				buf.WriteByte('?')
			} else {
				return "", fmt.Errorf("invalid escape sequence: %q", "\\"+string(c))
			}
		} else {
			buf.WriteRune(c)
		}
	}
	return buf.String(), nil
}

func (l *protoLex) skipToEndOfLineComment() (bool, string) {
	txt := []rune{'/', '/'}
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			return false, string(txt)
		}
		if c == '\n' {
			return true, string(append(txt, '\n'))
		}
		l.adjustPos(c)
		txt = append(txt, c)
	}
}

func (l *protoLex) skipToEndOfBlockComment() (string, bool) {
	txt := []rune{'/', '*'}
	for {
		c, _, err := l.input.readRune()
		if err != nil {
			return "", false
		}
		l.adjustPos(c)
		txt = append(txt, c)
		if c == '*' {
			c, _, err := l.input.readRune()
			if err != nil {
				return "", false
			}
			if c == '/' {
				l.adjustPos(c)
				txt = append(txt, c)
				return string(txt), true
			}
			l.input.unreadRune(c)
		}
	}
}

func (l *protoLex) addSourceError(err error) ErrorWithPos {
	ewp, ok := err.(ErrorWithPos)
	if !ok {
		ewp = ErrorWithSourcePos{Pos: l.prev(), Underlying: err}
	}
	_ = l.errs.handleError(ewp)
	return ewp
}

func (l *protoLex) Error(s string) {
	_ = l.addSourceError(errors.New(s))
}
