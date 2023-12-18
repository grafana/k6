package protoparse

import (
	"errors"
	"fmt"

	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"

	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// SourcePos is the same as ast.SourcePos. This alias exists for
// backwards compatibility (SourcePos used to be defined in this package.)
type SourcePos = ast.SourcePos

// ErrInvalidSource is a sentinel error that is returned by calls to
// Parser.ParseFiles and Parser.ParseFilesButDoNotLink in the event that syntax
// or link errors are encountered, but the parser's configured ErrorReporter
// always returns nil.
var ErrInvalidSource = reporter.ErrInvalidSource

// ErrNoSyntax is a sentinel error that may be passed to a warning reporter.
// The error the reporter receives will be wrapped with source position that
// indicates the file that had no syntax statement.
var ErrNoSyntax = parser.ErrNoSyntax

// ErrLookupImportAndProtoSet is the error returned if both LookupImport and LookupImportProto are set.
//
// Deprecated: This error is no longer used. It is now legal to set both LookupImport and LookupImportProto
// fields on the Parser.
var ErrLookupImportAndProtoSet = errors.New("both LookupImport and LookupImportProto set")

// ErrorReporter is responsible for reporting the given error. If the reporter
// returns a non-nil error, parsing/linking will abort with that error. If the
// reporter returns nil, parsing will continue, allowing the parser to try to
// report as many syntax and/or link errors as it can find.
type ErrorReporter = reporter.ErrorReporter

// WarningReporter is responsible for reporting the given warning. This is used
// for indicating non-error messages to the calling program for things that do
// not cause the parse to fail but are considered bad practice. Though they are
// just warnings, the details are supplied to the reporter via an error type.
type WarningReporter = reporter.WarningReporter

// ErrorWithPos is an error about a proto source file that includes information
// about the location in the file that caused the error.
//
// The value of Error() will contain both the SourcePos and Underlying error.
// The value of Unwrap() will only be the Underlying error.
type ErrorWithPos = reporter.ErrorWithPos

// ErrorWithSourcePos is an error about a proto source file that includes
// information about the location in the file that caused the error.
//
// Errors that include source location information *might* be of this type.
// However, calling code that is trying to examine errors with location info
// should instead look for instances of the ErrorWithPos interface, which
// will find other kinds of errors. This type is only exported for backwards
// compatibility.
//
// SourcePos should always be set and never nil.
type ErrorWithSourcePos struct {
	// These fields are present and exported for backwards-compatibility
	// with v1.4 and earlier.
	Underlying error
	Pos        *SourcePos

	reporter.ErrorWithPos
}

// Error implements the error interface
func (e ErrorWithSourcePos) Error() string {
	sourcePos := e.GetPosition()
	return fmt.Sprintf("%s: %v", sourcePos, e.Underlying)
}

// GetPosition implements the ErrorWithPos interface, supplying a location in
// proto source that caused the error.
func (e ErrorWithSourcePos) GetPosition() SourcePos {
	if e.Pos == nil {
		return SourcePos{Filename: "<input>"}
	}
	return *e.Pos
}

// Unwrap implements the ErrorWithPos interface, supplying the underlying
// error. This error will not include location information.
func (e ErrorWithSourcePos) Unwrap() error {
	return e.Underlying
}

var _ ErrorWithPos = ErrorWithSourcePos{}

func toErrorWithSourcePos(err ErrorWithPos) ErrorWithPos {
	pos := err.GetPosition()
	return ErrorWithSourcePos{
		ErrorWithPos: err,
		Underlying:   err.Unwrap(),
		Pos:          &pos,
	}
}

// ErrorUnusedImport may be passed to a warning reporter when an unused
// import is detected. The error the reporter receives will be wrapped
// with source position that indicates the file and line where the import
// statement appeared.
type ErrorUnusedImport = linker.ErrorUnusedImport

type errorWithFilename struct {
	underlying error
	filename   string
}

func (e errorWithFilename) Error() string {
	return fmt.Sprintf("%s: %v", e.filename, e.underlying)
}

func (e errorWithFilename) Unwrap() error {
	return e.underlying
}
