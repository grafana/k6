package common

import (
	"context"
)

type ctxKey int

const (
	ctxKeyBrowserOptions ctxKey = iota
	ctxKeyHooks
	ctxKeyIterationID
	ctxKeyTracer
)

func WithHooks(ctx context.Context, hooks *Hooks) context.Context {
	return context.WithValue(ctx, ctxKeyHooks, hooks)
}

func GetHooks(ctx context.Context) *Hooks {
	v := ctx.Value(ctxKeyHooks)
	if v == nil {
		return nil
	}
	return v.(*Hooks) //nolint:forcetypeassert
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

// WithBrowserOptions adds the browser options to the context.
func WithBrowserOptions(ctx context.Context, opts *BrowserOptions) context.Context {
	return context.WithValue(ctx, ctxKeyBrowserOptions, opts)
}

// GetBrowserOptions returns the browser options attached to the context.
func GetBrowserOptions(ctx context.Context) *BrowserOptions {
	v := ctx.Value(ctxKeyBrowserOptions)
	if v == nil {
		return nil
	}
	if bo, ok := v.(*BrowserOptions); ok {
		return bo
	}
	return nil
}

// WithTracer adds the given tracer to the context.
func WithTracer(ctx context.Context, tracer Tracer) context.Context {
	return context.WithValue(ctx, ctxKeyTracer, tracer)
}

// GetTracer returns the tracer attached to the context, or nil if not found.
func GetTracer(ctx context.Context) Tracer {
	v := ctx.Value(ctxKeyTracer)
	if v == nil {
		return nil
	}
	if tracer, ok := v.(Tracer); ok {
		return tracer
	}
	return nil
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
