package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	k6common "go.k6.io/k6/v2/js/common"
	k6modulestest "go.k6.io/k6/v2/js/modulestest"
	k6lib "go.k6.io/k6/v2/lib"
	k6metrics "go.k6.io/k6/v2/metrics"
)

func TestModuleVUBrowserRejectsInitContext(t *testing.T) {
	t.Parallel()

	vu := &k6modulestest.VU{
		RuntimeField: sobek.New(),
		InitEnvField: &k6common.InitEnvironment{
			TestPreInitState: &k6lib.TestPreInitState{
				Registry: k6metrics.NewRegistry(),
			},
		},
	}
	_, err := (moduleVU{VU: vu}).browser()
	require.ErrorIs(t, err, errBrowserInitContext)
}
