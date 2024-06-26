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

// Package reporter contains the types used for reporting errors from
// protocompile operations. It contains error types as well as interfaces
// for reporting and handling errors and warnings.
package reporter

import (
	"sync"

	"github.com/bufbuild/protocompile/ast"
)

// ErrorReporter is responsible for reporting the given error. If the reporter
// returns a non-nil error, parsing/linking will abort with that error. If the
// reporter returns nil, parsing will continue, allowing the parser to try to
// report as many syntax and/or link errors as it can find.
type ErrorReporter func(err ErrorWithPos) error

// WarningReporter is responsible for reporting the given warning. This is used
// for indicating non-error messages to the calling program for things that do
// not cause the parse to fail but are considered bad practice. Though they are
// just warnings, the details are supplied to the reporter via an error type.
type WarningReporter func(ErrorWithPos)

// Reporter is a type that handles reporting both errors and warnings.
// A reporter does not need to be thread-safe. Safe concurrent access is
// managed by a Handler.
type Reporter interface {
	// Error is called when the given error is encountered and needs to be
	// reported to the calling program. This signature matches ErrorReporter
	// because it has the same semantics. If this function returns non-nil
	// then the operation will abort immediately with the given error. But
	// if it returns nil, the operation will continue, reporting more errors
	// as they are encountered. If the reporter never returns non-nil then
	// the operation will eventually fail with ErrInvalidSource.
	Error(ErrorWithPos) error
	// Warning is called when the given warnings is encountered and needs to be
	// reported to the calling program. Despite the argument being an error
	// type, a warning will never cause the operation to abort or fail (unless
	// the reporter's implementation of this method panics).
	Warning(ErrorWithPos)
}

// NewReporter creates a new reporter that invokes the given functions on error
// or warning.
func NewReporter(errs ErrorReporter, warnings WarningReporter) Reporter {
	return reporterFuncs{errs: errs, warnings: warnings}
}

type reporterFuncs struct {
	errs     ErrorReporter
	warnings WarningReporter
}

func (r reporterFuncs) Error(err ErrorWithPos) error {
	if r.errs == nil {
		return err
	}
	return r.errs(err)
}

func (r reporterFuncs) Warning(err ErrorWithPos) {
	if r.warnings != nil {
		r.warnings(err)
	}
}

// Handler is used by protocompile operations for handling errors and warnings.
// This type is thread-safe. It uses a mutex to serialize calls to its reporter
// so that reporter instances do not have to be thread-safe (unless re-used
// across multiple handlers).
type Handler struct {
	parent       *Handler
	mu           sync.Mutex
	reporter     Reporter
	errsReported bool
	err          error
}

// NewHandler creates a new Handler that reports errors and warnings using the
// given reporter.
func NewHandler(rep Reporter) *Handler {
	if rep == nil {
		rep = NewReporter(nil, nil)
	}
	return &Handler{reporter: rep}
}

// SubHandler returns a "child" of h. Use of a child handler is the same as use
// of the parent, except that the Error() and ReporterError() functions only
// report non-nil for errors that were reported using the child handler. So
// errors reported directly to the parent or to a different child handler won't
// be returned. This is useful for making concurrent access to the handler more
// deterministic: if a child handler is only used from one goroutine, its view
// of reported errors is consistent and unimpacted by concurrent operations.
func (h *Handler) SubHandler() *Handler {
	return &Handler{parent: h}
}

// HandleError handles the given error. If the given err is an ErrorWithPos, it
// is reported, and this function returns the error returned by the reporter. If
// the given err is NOT an ErrorWithPos, the current operation will abort
// immediately.
//
// If the handler has already aborted (by returning a non-nil error from a prior
// call to HandleError or HandleErrorf), that same error is returned and the
// given error is not reported.
func (h *Handler) HandleError(err error) error {
	if h.parent != nil {
		_, isErrWithPos := err.(ErrorWithPos)
		err = h.parent.HandleError(err)

		// update child state
		h.mu.Lock()
		defer h.mu.Unlock()
		if isErrWithPos {
			h.errsReported = true
		}
		h.err = err
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.err != nil {
		return h.err
	}
	if ewp, ok := err.(ErrorWithPos); ok {
		h.errsReported = true
		err = h.reporter.Error(ewp)
	}
	h.err = err
	return err
}

// HandleErrorWithPos handles an error with the given source position.
//
// If the handler has already aborted (by returning a non-nil error from a prior
// call to HandleError or HandleErrorf), that same error is returned and the
// given error is not reported.
func (h *Handler) HandleErrorWithPos(span ast.SourceSpan, err error) error {
	if ewp, ok := err.(ErrorWithPos); ok {
		// replace existing position with given one
		err = errorWithSpan{SourceSpan: span, underlying: ewp.Unwrap()}
	} else {
		err = errorWithSpan{SourceSpan: span, underlying: err}
	}
	return h.HandleError(err)
}

// HandleErrorf handles an error with the given source position, creating the
// error using the given message format and arguments.
//
// If the handler has already aborted (by returning a non-nil error from a call
// to HandleError or HandleErrorf), that same error is returned and the given
// error is not reported.
func (h *Handler) HandleErrorf(span ast.SourceSpan, format string, args ...interface{}) error {
	return h.HandleError(Errorf(span, format, args...))
}

// HandleWarning handles the given warning. This will delegate to the handler's
// configured reporter.
func (h *Handler) HandleWarning(err ErrorWithPos) {
	if h.parent != nil {
		h.parent.HandleWarning(err)
		return
	}

	// even though we aren't touching mutable fields, we acquire lock anyway so
	// that underlying reporter does not have to be thread-safe
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reporter.Warning(err)
}

// HandleWarningWithPos handles a warning with the given source position. This will
// delegate to the handler's configured reporter.
func (h *Handler) HandleWarningWithPos(span ast.SourceSpan, err error) {
	ewp, ok := err.(ErrorWithPos)
	if ok {
		// replace existing position with given one
		ewp = errorWithSpan{SourceSpan: span, underlying: ewp.Unwrap()}
	} else {
		ewp = errorWithSpan{SourceSpan: span, underlying: err}
	}
	h.HandleWarning(ewp)
}

// HandleWarningf handles a warning with the given source position, creating the
// actual error value using the given message format and arguments.
func (h *Handler) HandleWarningf(span ast.SourceSpan, format string, args ...interface{}) {
	h.HandleWarning(Errorf(span, format, args...))
}

// Error returns the handler result. If any errors have been reported then this
// returns a non-nil error. If the reporter never returned a non-nil error then
// ErrInvalidSource is returned. Otherwise, this returns the error returned by
// the  handler's reporter (the same value returned by ReporterError).
func (h *Handler) Error() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.errsReported && h.err == nil {
		return ErrInvalidSource
	}
	return h.err
}

// ReporterError returns the error returned by the handler's reporter. If
// the reporter has either not been invoked (no errors handled) or has not
// returned any non-nil value, then this returns nil.
func (h *Handler) ReporterError() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.err
}
