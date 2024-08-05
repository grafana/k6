package flameql

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidQuerySyntax    = errors.New("invalid query syntax")
	ErrInvalidAppName        = errors.New("invalid application name")
	ErrInvalidMatchersSyntax = errors.New("invalid tag matchers syntax")
	ErrInvalidTagKey         = errors.New("invalid tag key")
	ErrInvalidTagValueSyntax = errors.New("invalid tag value syntax")

	ErrAppNameIsRequired = errors.New("application name is required")
	ErrTagKeyIsRequired  = errors.New("tag key is required")
	ErrTagKeyReserved    = errors.New("tag key is reserved")

	ErrMatchOperatorIsRequired = errors.New("match operator is required")
	ErrUnknownOp               = errors.New("unknown tag match operator")
)

type Error struct {
	Inner error
	Expr  string
	// TODO: add offset?
}

func newErr(err error, expr string) *Error { return &Error{Inner: err, Expr: expr} }

func (e *Error) Error() string { return e.Inner.Error() + ": " + e.Expr }

func (e *Error) Unwrap() error { return e.Inner }

func newInvalidTagKeyRuneError(k string, r rune) *Error {
	return newInvalidRuneError(ErrInvalidTagKey, k, r)
}

func newInvalidAppNameRuneError(k string, r rune) *Error {
	return newInvalidRuneError(ErrInvalidAppName, k, r)
}

func newInvalidRuneError(err error, k string, r rune) *Error {
	return newErr(err, fmt.Sprintf("%s: character is not allowed: %q", k, r))
}
