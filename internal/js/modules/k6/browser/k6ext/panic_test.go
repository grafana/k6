package k6ext_test

import (
	"errors"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestPanicfDoesNotTouchSobekRuntime(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		k6ext.Panicf(vu.Context(), "attaching to target: %w", errors.New("canceled"))
	}()

	require.NotNil(t, recovered)
	_, isException := recovered.(*sobek.Exception)
	assert.False(t, isException, "Panicf must not wrap through Sobek off the event loop")
	err, ok := recovered.(error)
	require.True(t, ok)
	assert.ErrorContains(t, err, "attaching to target: canceled")
}
