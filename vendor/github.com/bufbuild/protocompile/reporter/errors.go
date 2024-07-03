// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"errors"
	"fmt"

	"github.com/bufbuild/protocompile/ast"
)

// ErrInvalidSource is a sentinel error that is returned by compilation and
// stand-alone compilation steps (such as parsing, linking) when one or more
// errors is reported but the configured ErrorReporter always returns nil.
var ErrInvalidSource = errors.New("parse failed: invalid proto source")

// ErrorWithPos is an error about a proto source file that adds information
// about the location in the file that caused the error.
type ErrorWithPos interface {
	error
	ast.SourceSpan
	// GetPosition returns the start source position that caused the underlying error.
	GetPosition() ast.SourcePos
	// Unwrap returns the underlying error.
	Unwrap() error
}

// Error creates a new ErrorWithPos from the given error and source position.
func Error(span ast.SourceSpan, err error) ErrorWithPos {
	return errorWithSpan{SourceSpan: span, underlying: err}
}

// Errorf creates a new ErrorWithPos whose underlying error is created using the
// given message format and arguments (via fmt.Errorf).
func Errorf(span ast.SourceSpan, format string, args ...interface{}) ErrorWithPos {
	return errorWithSpan{SourceSpan: span, underlying: fmt.Errorf(format, args...)}
}

type errorWithSpan struct {
	ast.SourceSpan
	underlying error
}

func (e errorWithSpan) Error() string {
	sourcePos := e.GetPosition()
	return fmt.Sprintf("%s: %v", sourcePos, e.underlying)
}

func (e errorWithSpan) GetPosition() ast.SourcePos {
	return e.Start()
}

func (e errorWithSpan) Unwrap() error {
	return e.underlying
}

var _ ErrorWithPos = errorWithSpan{}
