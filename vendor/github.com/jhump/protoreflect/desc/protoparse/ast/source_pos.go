package ast

import (
	"github.com/bufbuild/protocompile/ast"
)

// SourcePos identifies a location in a proto source file.
type SourcePos = ast.SourcePos

// PosRange is a range of positions in a source file that indicates
// the span of some region of source, such as a single token or
// a sub-tree of the AST.
type PosRange struct {
	Start, End SourcePos
}

// Comment represents a single comment in a source file. It indicates
// the position of the comment and its contents.
type Comment struct {
	// The location of the comment in the source file.
	PosRange
	// Any whitespace between the prior lexical element (either a token
	// or other comment) and this comment.
	LeadingWhitespace string
	// The text of the comment, including any "//" or "/*" and "*/"
	// symbols at the start and end. Single-line comments will include
	// the trailing newline rune in Text.
	Text string
}
