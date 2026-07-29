package streams

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestHostPromiseReactionPreservesMicrotaskOrdering(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	events := make([]string, 0, 4)
	require.NoError(t, rt.Set("record", func(event string) { events = append(events, event) }))

	jsCallbackValue, err := rt.RunString(`
() => {
  record("javascript callback");
  Promise.resolve().then(() => record("nested microtask"));
}
`)
	require.NoError(t, err)
	jsCallback, ok := sobek.AssertFunction(jsCallbackValue)
	require.True(t, ok)

	resolvedValue, err := rt.RunString(`Promise.resolve()`)
	require.NoError(t, err)
	resolved, ok := resolvedValue.Export().(*sobek.Promise)
	require.True(t, ok)

	resolved.Then(func(sobek.Value) sobek.Value {
		events = append(events, "host reaction start")
		if _, callErr := jsCallback(sobek.Undefined()); callErr != nil {
			panic(callErr)
		}
		events = append(events, "host reaction end")
		return sobek.Undefined()
	}, nil)

	// Entering and leaving the runtime drains the queued host reaction and its nested microtask.
	_, err = rt.RunString(`undefined`)
	require.NoError(t, err)
	require.Equal(t, []string{
		"host reaction start",
		"javascript callback",
		"host reaction end",
		"nested microtask",
	}, events)
}
