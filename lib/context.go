package lib

import "context"

type ctxKey int

const (
	ctxKeyState ctxKey = iota
)

func WithState(ctx context.Context, state *State) context.Context {
	return context.WithValue(ctx, ctxKeyState, state)
}

func GetState(ctx context.Context) *State {
	v := ctx.Value(ctxKeyState)
	if v == nil {
		return nil
	}
	return v.(*State)
}
