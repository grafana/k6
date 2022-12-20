/*
Package parser implements a parser for JavaScript.

	import (
	    "github.com/dop251/goja/parser"
	)

Parse and return an AST

	filename := "" // A filename is optional
	src := `
	    // Sample xyzzy example
	    (function(){
	        if (3.14159 > 0) {
	            console.log("Hello, World.");
	            return;
	        }

	        var xyzzy = NaN;
	        console.log("Nothing happens.");
	        return xyzzy;
	    })();
	`

	// Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
	program, err := parser.ParseFile(nil, filename, src, 0)

# Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.
*/
package parser

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/token"
	"github.com/dop251/goja/unistring"
)

// A Mode value is a set of flags (or 0). They control optional parser functionality.
type Mode uint

const (
	IgnoreRegExpErrors Mode = 1 << iota // Ignore RegExp compatibility errors (allow backtracking)
)

type options struct {
	disableSourceMaps bool
	sourceMapLoader   func(path string) ([]byte, error)
}

// Option represents one of the options for the parser to use in the Parse methods. Currently supported are:
// WithDisableSourceMaps and WithSourceMapLoader.
type Option func(*options)

// WithDisableSourceMaps is an option to disable source maps support. May save a bit of time when source maps
// are not in use.
func WithDisableSourceMaps(opts *options) {
	opts.disableSourceMaps = true
}

// WithSourceMapLoader is an option to set a custom source map loader. The loader will be given a path or a
// URL from the sourceMappingURL. If sourceMappingURL is not absolute it is resolved relatively to the name
// of the file being parsed. Any error returned by the loader will fail the parsing.
// Note that setting this to nil does not disable source map support, there is a default loader which reads
// from the filesystem. Use WithDisableSourceMaps to disable source map support.
func WithSourceMapLoader(loader func(path string) ([]byte, error)) Option {
	return func(opts *options) {
		opts.sourceMapLoader = loader
	}
}

type _parser struct {
	str    string
	length int
	base   int

	chr       rune // The current character
	chrOffset int  // The offset of current character
	offset    int  // The offset after current character (may be greater than 1)

	idx           file.Idx    // The index of token
	token         token.Token // The token
	literal       string      // The literal of the token, if any
	parsedLiteral unistring.String

	scope             *_scope
	insertSemicolon   bool // If we see a newline, then insert an implicit semicolon
	implicitSemicolon bool // An implicit semicolon exists

	errors ErrorList

	recover struct {
		// Scratch when trying to seek to the next statement, etc.
		idx   file.Idx
		count int
	}

	mode Mode
	opts options

	file *file.File
}

func _newParser(filename, src string, base int, opts ...Option) *_parser {
	p := &_parser{
		chr:    ' ', // This is set so we can start scanning by skipping whitespace
		str:    src,
		length: len(src),
		base:   base,
		file:   file.NewFile(filename, src, base),
	}
	for _, opt := range opts {
		opt(&p.opts)
	}
	return p
}

func newParser(filename, src string) *_parser {
	return _newParser(filename, src, 1)
}

func ReadSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch src := src.(type) {
		case string:
			return []byte(src), nil
		case []byte:
			return src, nil
		case *bytes.Buffer:
			if src != nil {
				return src.Bytes(), nil
			}
		case io.Reader:
			var bfr bytes.Buffer
			if _, err := io.Copy(&bfr, src); err != nil {
				return nil, err
			}
			return bfr.Bytes(), nil
		}
		return nil, errors.New("invalid source")
	}
	return os.ReadFile(filename)
}

// ParseFile parses the source code of a single JavaScript/ECMAScript source file and returns
// the corresponding ast.Program node.
//
// If fileSet == nil, ParseFile parses source without a FileSet.
// If fileSet != nil, ParseFile first adds filename and src to fileSet.
//
// The filename argument is optional and is used for labelling errors, etc.
//
// src may be a string, a byte slice, a bytes.Buffer, or an io.Reader, but it MUST always be in UTF-8.
//
//	// Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
//	program, err := parser.ParseFile(nil, "", `if (abc > 1) {}`, 0)
func ParseFile(fileSet *file.FileSet, filename string, src interface{}, mode Mode, options ...Option) (*ast.Program, error) {
	str, err := ReadSource(filename, src)
	if err != nil {
		return nil, err
	}
	{
		str := string(str)

		base := 1
		if fileSet != nil {
			base = fileSet.AddFile(filename, str)
		}

		parser := _newParser(filename, str, base, options...)
		parser.mode = mode
		return parser.parse()
	}
}

// ParseFunction parses a given parameter list and body as a function and returns the
// corresponding ast.FunctionLiteral node.
//
// The parameter list, if any, should be a comma-separated list of identifiers.
func ParseFunction(parameterList, body string, options ...Option) (*ast.FunctionLiteral, error) {

	src := "(function(" + parameterList + ") {\n" + body + "\n})"

	parser := _newParser("", src, 1, options...)
	program, err := parser.parse()
	if err != nil {
		return nil, err
	}

	return program.Body[0].(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral), nil
}

func (self *_parser) slice(idx0, idx1 file.Idx) string {
	from := int(idx0) - self.base
	to := int(idx1) - self.base
	if from >= 0 && to <= len(self.str) {
		return self.str[from:to]
	}

	return ""
}

func (self *_parser) parse() (*ast.Program, error) {
	self.openScope()
	defer self.closeScope()
	self.next()
	program := self.parseProgram()
	if false {
		self.errors.Sort()
	}
	return program, self.errors.Err()
}

func (self *_parser) next() {
	self.token, self.literal, self.parsedLiteral, self.idx = self.scan()
}

func (self *_parser) optionalSemicolon() {
	if self.token == token.SEMICOLON {
		self.next()
		return
	}

	if self.implicitSemicolon {
		self.implicitSemicolon = false
		return
	}

	if self.token != token.EOF && self.token != token.RIGHT_BRACE {
		self.expect(token.SEMICOLON)
	}
}

func (self *_parser) semicolon() {
	if self.token != token.RIGHT_PARENTHESIS && self.token != token.RIGHT_BRACE {
		if self.implicitSemicolon {
			self.implicitSemicolon = false
			return
		}

		self.expect(token.SEMICOLON)
	}
}

func (self *_parser) idxOf(offset int) file.Idx {
	return file.Idx(self.base + offset)
}

func (self *_parser) expect(value token.Token) file.Idx {
	idx := self.idx
	if self.token != value {
		self.errorUnexpectedToken(self.token)
	}
	self.next()
	return idx
}

func (self *_parser) position(idx file.Idx) file.Position {
	return self.file.Position(int(idx) - self.base)
}
