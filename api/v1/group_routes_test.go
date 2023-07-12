package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/minirunner"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
)

func getTestPreInitState(tb testing.TB) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	logger := testutils.NewLogger(tb)
	return &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}
}

func getTestRunState(tb testing.TB, options lib.Options, runner lib.Runner) *lib.TestRunState {
	require.NoError(tb, runner.SetOptions(runner.GetOptions().Apply(options)))
	piState := getTestPreInitState(tb)
	return &lib.TestRunState{
		TestPreInitState: piState,
		Options:          options,
		Runner:           runner,
		RunTags:          piState.Registry.RootTagSet().WithTagsFromMap(options.RunTags),
	}
}

func getControlSurface(tb testing.TB, testState *lib.TestRunState) *ControlSurface {
	execScheduler, err := execution.NewScheduler(testState)
	require.NoError(tb, err)

	me, err := engine.NewMetricsEngine(testState.Registry, testState.Logger)
	require.NoError(tb, err)

	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)
	ctx, _ = execution.NewTestRunContext(ctx, testState.Logger)

	return &ControlSurface{
		RunCtx:        ctx,
		Samples:       make(chan metrics.SampleContainer, 1000),
		MetricsEngine: me,
		Scheduler:     execScheduler,
		RunState:      testState,
	}
}

func TestGetGroups(t *testing.T) {
	g0, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	g1, err := g0.Group("group 1")
	assert.NoError(t, err)
	g2, err := g1.Group("group 2")
	assert.NoError(t, err)

	cs := getControlSurface(t, getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{Group: g0}))

	t.Run("list", func(t *testing.T) {
		rw := httptest.NewRecorder()
		NewHandler(cs).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v1/groups", nil))
		res := rw.Result()
		body := rw.Body.Bytes()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.NotEmpty(t, body)

		t.Run("document", func(t *testing.T) {
			var doc groupsJSONAPI
			assert.NoError(t, json.Unmarshal(body, &doc))
			if assert.NotEmpty(t, doc.Data) {
				assert.Equal(t, "groups", doc.Data[0].Type)
			}
		})

		t.Run("groups", func(t *testing.T) {
			var envelop groupsJSONAPI
			require.NoError(t, json.Unmarshal(body, &envelop))
			require.Len(t, envelop.Data, 3)

			for _, data := range envelop.Data {
				current := data.Attributes

				switch current.ID {
				case g0.ID:
					assert.Equal(t, "", current.Name)
					assert.Nil(t, current.Parent)
					assert.Equal(t, "", current.ParentID)
					assert.Len(t, current.GroupIDs, 1)
					assert.EqualValues(t, []string{g1.ID}, current.GroupIDs)
				case g1.ID:
					assert.Equal(t, "group 1", current.Name)
					assert.Nil(t, current.Parent)
					assert.Equal(t, g0.ID, current.ParentID)
					assert.EqualValues(t, []string{g2.ID}, current.GroupIDs)
				case g2.ID:
					assert.Equal(t, "group 2", current.Name)
					assert.Nil(t, current.Parent)
					assert.Equal(t, g1.ID, current.ParentID)
					assert.EqualValues(t, []string{}, current.GroupIDs)
				default:
					assert.Fail(t, "Unknown ID: "+current.ID)
				}
			}
		})
	})
	for _, gp := range []*lib.Group{g0, g1, g2} {
		t.Run(gp.Name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			NewHandler(cs).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v1/groups/"+gp.ID, nil))
			res := rw.Result()
			assert.Equal(t, http.StatusOK, res.StatusCode)
		})
	}
}
