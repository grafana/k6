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

func newFrameMappingTestVU() moduleVU {
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

func TestMapFrameNilReturnsNil(t *testing.T) {
	t.Parallel()

	require.Nil(t, mapFrame(newFrameMappingTestVU(), nil))
}

func TestFrameParentFrameMainFrameReturnsNil(t *testing.T) {
	t.Parallel()

	vu := newFrameMappingTestVU()
	// A zero Frame is a main frame: ParentFrame() is nil.
	m := mapFrame(vu, &common.Frame{})
	parentFrame, ok := m["parentFrame"].(func() mapping)
	require.True(t, ok, "parentFrame should be mapped as func() mapping")

	// Playwright's frame.parentFrame() is null for the main frame.
	// Returning a mapped object here used to panic on the first method
	// call (e.g. name / evaluate / click).
	require.Nil(t, parentFrame())

	rt := vu.Runtime()
	require.NoError(t, rt.Set("frame", mapToSobek(vu, m)))
	v, err := rt.RunString(`frame.parentFrame()`)
	require.NoError(t, err)
	require.True(t, sobek.IsNull(v) || sobek.IsUndefined(v), "got %v", v)
	truth, err := rt.RunString(`frame.parentFrame() ? 'truthy' : 'falsy'`)
	require.NoError(t, err)
	require.Equal(t, "falsy", truth.String())
}

func TestRequestFrameNilReturnsNil(t *testing.T) {
	t.Parallel()

	vu := newFrameMappingTestVU()
	// Requests without a FrameID (service workers, detached frames)
	// leave Request.frame nil. Playwright's request.frame() is null.
	m := mapRequest(vu, &common.Request{})
	frameFn, ok := m["frame"].(func() any)
	require.True(t, ok, "frame should be mapped as func() any")
	require.Nil(t, frameFn())

	rt := vu.Runtime()
	require.NoError(t, rt.Set("req", mapToSobek(vu, m)))
	v, err := rt.RunString(`req.frame()`)
	require.NoError(t, err)
	require.True(t, sobek.IsNull(v) || sobek.IsUndefined(v), "got %v", v)
	truth, err := rt.RunString(`req.frame() ? 'truthy' : 'falsy'`)
	require.NoError(t, err)
	require.Equal(t, "falsy", truth.String())
}
