package lib

import (
	"context"
	"errors"
	"fmt"
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

	return v.(*ExecutionState) //nolint:forcetypeassert
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
	return v.(*ScenarioState) //nolint:forcetypeassert
}

// ContextErr returns ctx.Err() and, if present, appends the cancel cause.
func ContextErr(ctx context.Context) error {
	err := ctx.Err()
	if err == nil {
		return nil
	}

	cause := context.Cause(ctx)
	if cause == nil || errors.Is(cause, err) {
		return err
	}

	return fmt.Errorf("%w: %w", err, cause)
}
