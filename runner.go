package speedboat

import (
	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	AbortTest FlowControl = 0

	LoggerKey ContextKey = 0
)

type FlowControl int

type ContextKey int

func (op FlowControl) Error() string {
	switch op {
	case 0:
		return "OP: Abort Test"
	default:
		return "Unknown flow control OP"
	}
}

func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

func GetLogger(ctx context.Context) *log.Logger {
	return ctx.Value(LoggerKey).(*log.Logger)
}

type Runner interface {
	RunVU(ctx context.Context, t Test, id int)
}
