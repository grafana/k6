package lib

import (
	"context"
)

type ctxKey int

const (
	ctxKeyExecState ctxKey = iota
	ctxKeyScenario
)

// WithExecutionState embeds an ExecutionState in ctx.
func WithExecutionState(ctx context.Context, s *ExecutionState) context.Context {
	return context.WithValue(ctx, ctxKeyExecState, s)
}

// GetExecutionState returns an ExecutionState from ctx.
func GetExecutionState(ctx context.Context) *ExecutionState {
	v := ctx.Value(ctxKeyExecState)
	if v == nil {
		return nil
	}
	return v.(*ExecutionState)
}

// WithScenarioState embeds a ScenarioState in ctx.
func WithScenarioState(ctx context.Context, s *ScenarioState) context.Context {
	return context.WithValue(ctx, ctxKeyScenario, s)
}

// GetScenarioState returns a ScenarioState from ctx.
func GetScenarioState(ctx context.Context) *ScenarioState {
	v := ctx.Value(ctxKeyScenario)
	if v == nil {
		return nil
	}
	return v.(*ScenarioState)
}
