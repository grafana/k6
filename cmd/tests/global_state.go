// Package tests contains types needed for running integration tests that run k6 commands.
package tests

import (
	"testing"

	"go.k6.io/k6/internal/cmd/tests"
)

// GlobalTestState is a wrapper around GlobalState for use in tests.
type GlobalTestState = tests.GlobalTestState

// NewGlobalTestState returns an initialized GlobalTestState, mocking all
// GlobalState fields for use in tests.
func NewGlobalTestState(tb testing.TB) *GlobalTestState {
	return tests.NewGlobalTestState(tb)
}
