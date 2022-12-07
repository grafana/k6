// Package errext contains extensions for normal Go errors that are used in k6.
package errext

// Exception represents errors that resulted from a script exception and contain
// a stack trace that lead to them.
type Exception interface {
	error
	HasAbortReason
	StackTrace() string
}
