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

type Runner interface {
	NewVU() (VU, error)
}

type VU interface {
	Reconfigure(id int64) error
	RunOnce(ctx context.Context) error
}

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
