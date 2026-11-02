package common

import (
	"context"
	"math"
	"time"

	"github.com/chromedp/cdproto/cdp"
)

type executor interface {
	Execute(context.Context, string, any, any) error
}

func ptr[T any](v T) *T { return &v }

// protocolCommand binds protocol parameters to a command for deferred execution.
type protocolCommand[P, R any] struct {
	method string
	params P
}

func newCommand[P, R any](c cdp.Command[P, R], p P) protocolCommand[P, R] {
	return protocolCommand[P, R]{c.Method, p}
}
func (c protocolCommand[P, R]) Do(ctx context.Context, e executor) error {
	return c.execute(ctx, e, nil)
}
func (c protocolCommand[P, R]) Result(ctx context.Context, e executor) (R, error) {
	var r R
	err := e.Execute(ctx, c.method, &c.params, &r)
	return r, err
}

func (c protocolCommand[P, R]) execute(ctx context.Context, e executor, result any) error {
	return e.Execute(ctx, c.method, &c.params, result)
}

func protocolTime(seconds float64) time.Time {
	whole, fraction := math.Modf(seconds)
	return time.Unix(int64(whole), int64(fraction*1e9))
}
