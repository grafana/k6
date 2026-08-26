package icmp

import (
	"errors"
	"fmt"
)

var (
	errUnexpectedMessage = errors.New("unexpected ICMP message")
	errUnexpectedSource  = errors.New("unexpected source address")
	errDeadlineExceeded  = errors.New("deadline exceeded")
	errInvalidType       = errors.New("invalid type")
)

type icmpError struct {
	Name    string
	Message string
}

func wrapError(err error) *icmpError {
	return &icmpError{
		Name:    "ICMPError",
		Message: err.Error(),
	}
}

func (e *icmpError) Error() string {
	return fmt.Sprintf("ICMP error: %v", e.Message)
}
