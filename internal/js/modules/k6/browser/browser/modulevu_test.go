package browser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

// TestModuleVUBrowserInInitContext ensures that resolving the VU browser in the
// init context returns errInitContext instead of nil-dereferencing VU.State().
// See #6178.
func TestModuleVUBrowserInInitContext(t *testing.T) {
	t.Parallel()

	// A VU that has not been activated has a nil State: the init context.
	vu := k6test.NewVU(t)

	_, err := moduleVU{VU: vu}.browser()
	require.ErrorIs(t, err, errInitContext)
}
