package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"

	k6common "go.k6.io/k6/v2/js/common"
	k6modulestest "go.k6.io/k6/v2/js/modulestest"
	k6lib "go.k6.io/k6/v2/lib"
	k6metrics "go.k6.io/k6/v2/metrics"
)

func newMappingTestVU() moduleVU {
	return moduleVU{
		VU: &k6modulestest.VU{
			RuntimeField: sobek.New(),
			InitEnvField: &k6common.InitEnvironment{
				TestPreInitState: &k6lib.TestPreInitState{
					Registry: k6metrics.NewRegistry(),
				},
			},
		},
	}
}

func TestJSHandleAsElementNonElementReturnsNil(t *testing.T) {
	t.Parallel()

	m := mapJSHandle(newMappingTestVU(), &common.BaseJSHandle{})
	asElement, ok := m["asElement"].(func() any)
	require.True(t, ok, "asElement should be mapped as func() any")

	// A handle to a primitive/object (not a DOM node) must be null in JS,
	// matching Playwright. Returning a mapped object here used to panic
	// on the first method call (e.g. click / boundingBox).
	require.Nil(t, asElement())
}

func TestJSHandleAsElementOnElementHandle(t *testing.T) {
	t.Parallel()

	eh := &common.ElementHandle{}
	m := mapJSHandle(newMappingTestVU(), eh)
	asElement, ok := m["asElement"].(func() any)
	require.True(t, ok)

	got := asElement()
	require.NotNil(t, got)
	em, ok := got.(mapping)
	require.True(t, ok)
	require.Contains(t, em, "click")
	require.Contains(t, em, "boundingBox")
}

func TestMapElementHandleNilPanicsOnUse(t *testing.T) {
	t.Parallel()

	// Guardrail: mapping a nil *ElementHandle must not be exposed to JS.
	// Methods that parse options call eh.DefaultTimeout() immediately.
	m := mapElementHandle(newMappingTestVU(), nil)
	check, ok := m["check"].(func(sobek.Value) (*sobek.Promise, error))
	require.True(t, ok)

	require.Panics(t, func() {
		_, _ = check(sobek.Undefined())
	})
}
