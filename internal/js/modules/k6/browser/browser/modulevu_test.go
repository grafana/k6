package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	k6modulestest "go.k6.io/k6/v2/js/modulestest"
)

func TestBrowserWithoutIterationState(t *testing.T) {
	t.Parallel()

	vu := moduleVU{VU: &k6modulestest.VU{RuntimeField: sobek.New()}}

	_, err := vu.browser()
	require.ErrorIs(t, err, errBrowserInitContext)
}
