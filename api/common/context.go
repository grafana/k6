package common

import (
	"context"

	"go.k6.io/k6/core"
)

type ContextKey int

const ctxKeyEngine = ContextKey(1)

// WithEngine sets the k6 running Engine in the under the hood context.
//
// Deprecated: Use directly the Engine as dependency.
func WithEngine(ctx context.Context, engine *core.Engine) context.Context {
	return context.WithValue(ctx, ctxKeyEngine, engine)
}

// GetEngine returns the k6 running Engine fetching it from the context.
//
// Deprecated: Use directly the Engine as dependency.
func GetEngine(ctx context.Context) *core.Engine {
	return ctx.Value(ctxKeyEngine).(*core.Engine)
}
