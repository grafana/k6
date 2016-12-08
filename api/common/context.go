package common

import (
	"context"
	"github.com/loadimpact/k6/lib"
)

type ContextKey int

const ctxKeyEngine = ContextKey(1)

func WithEngine(ctx context.Context, engine *lib.Engine) context.Context {
	return context.WithValue(ctx, ctxKeyEngine, engine)
}

func GetEngine(ctx context.Context) *lib.Engine {
	return ctx.Value(ctxKeyEngine).(*lib.Engine)
}
