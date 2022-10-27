package common

import (
	"context"
)

type ctxKey int

const (
	ctxKeyLaunchOptions ctxKey = iota
	ctxKeyHooks
	ctxKeyIterationID
)

func WithHooks(ctx context.Context, hooks *Hooks) context.Context {
	return context.WithValue(ctx, ctxKeyHooks, hooks)
}

func GetHooks(ctx context.Context) *Hooks {
	v := ctx.Value(ctxKeyHooks)
	if v == nil {
		return nil
	}
	return v.(*Hooks)
}

// WithIterationID adds an identifier for the current iteration to the context.
func WithIterationID(ctx context.Context, iterID string) context.Context {
	return context.WithValue(ctx, ctxKeyIterationID, iterID)
}

// GetIterationID returns the iteration identifier attached to the context.
func GetIterationID(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyIterationID).(string)
	return s
}

func WithLaunchOptions(ctx context.Context, opts *LaunchOptions) context.Context {
	return context.WithValue(ctx, ctxKeyLaunchOptions, opts)
}

func GetLaunchOptions(ctx context.Context) *LaunchOptions {
	v := ctx.Value(ctxKeyLaunchOptions)
	if v == nil {
		return nil
	}
	return v.(*LaunchOptions)
}

// contextWithDoneChan returns a new context that is canceled either
// when the done channel is closed or ctx is canceled.
func contextWithDoneChan(ctx context.Context, done chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		select {
		case <-done:
		case <-ctx.Done():
		}
	}()
	return ctx
}
