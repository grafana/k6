package speedboat

import (
	"golang.org/x/net/context"
)

const (
	AbortTest FlowControl = 0
)

type FlowControl int

func (op FlowControl) Error() string {
	switch op {
	case 0:
		return "OP: Abort Test"
	default:
		return "Unknown flow control OP"
	}
}

type Runner interface {
	RunVU(ctx context.Context, t Test, id int)
}
