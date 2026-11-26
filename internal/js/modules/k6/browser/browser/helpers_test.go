package browser

import (
	"context"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k6modulestest "go.k6.io/k6/v2/js/modulestest"
)

func TestPromiseWithoutIterationState(t *testing.T) {
	t.Parallel()

	vu := moduleVU{VU: &k6modulestest.VU{
		CtxField:     context.Background(),
		RuntimeField: sobek.New(),
		RegisterCallbackField: func() func(f func() error) {
			return func(f func() error) { _ = f() }
		},
	}}

	promise := promise(vu, func() (any, error) {
		t.Fatal("promise callback ran in the init context")
		return nil, nil
	})

	require.Equal(t, sobek.PromiseStateRejected, promise.State())
	assert.Contains(t, promise.Result().String(), "iteration context")
}
