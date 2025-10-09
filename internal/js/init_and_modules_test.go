package js_test

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
)

type CheckModule struct {
	t             testing.TB
	initCtxCalled int
	vuCtxCalled   int
}

func (cm *CheckModule) InitCtx(_ context.Context) {
	cm.initCtxCalled++
}

func (cm *CheckModule) VuCtx(_ context.Context) {
	cm.vuCtxCalled++
}

var uniqueModuleNumber int64 //nolint:gochecknoglobals // we need this so multiple test can register differently named modules

func TestNewJSRunnerWithCustomModule(t *testing.T) {
	t.Parallel()

	checkModule := &CheckModule{t: t}
	moduleName := fmt.Sprintf("k6/x/check-%d", atomic.AddInt64(&uniqueModuleNumber, 1))
	modules.Register(moduleName, checkModule)

	script := fmt.Sprintf(`
		var check = require("%s");
		check.initCtx();

		module.exports.options = { vus: 1, iterations: 1 };
		module.exports.default = function() {
			check.vuCtx();
		};
	`, moduleName)

	logger := testutils.NewLogger(t)
	rtOptions := lib.RuntimeOptions{CompatibilityMode: null.StringFrom("base")}
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := js.New(
		&lib.TestPreInitState{
			Logger:         logger,
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
			RuntimeOptions: rtOptions,
			Usage:          usage.New(),
		},
		&loader.SourceData{
			URL:  &url.URL{Path: "blah", Scheme: "file"},
			Data: []byte(script),
		},
		map[string]fsext.Fs{"file": fsext.NewMemMapFs(), "https": fsext.NewMemMapFs()},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, checkModule.initCtxCalled)
	assert.Equal(t, 0, checkModule.vuCtxCalled)

	vu, err := runner.NewVU(t.Context(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	assert.Equal(t, 2, checkModule.initCtxCalled)
	assert.Equal(t, 0, checkModule.vuCtxCalled)

	vuCtx, vuCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer vuCancel()

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: vuCtx})
	require.NoError(t, activeVU.RunOnce())
	assert.Equal(t, 2, checkModule.initCtxCalled)
	assert.Equal(t, 1, checkModule.vuCtxCalled)
	require.NoError(t, activeVU.RunOnce())
	assert.Equal(t, 2, checkModule.initCtxCalled)
	assert.Equal(t, 2, checkModule.vuCtxCalled)

	arc := runner.MakeArchive()
	assert.Equal(t, 2, checkModule.initCtxCalled) // shouldn't change, we're not executing the init context again
	assert.Equal(t, 2, checkModule.vuCtxCalled)

	runnerFromArc, err := js.NewFromArchive(
		&lib.TestPreInitState{
			Logger:         logger,
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
			RuntimeOptions: rtOptions,
			Usage:          usage.New(),
		}, arc)
	require.NoError(t, err)
	assert.Equal(t, 3, checkModule.initCtxCalled) // changes because we need to get the exported functions
	assert.Equal(t, 2, checkModule.vuCtxCalled)
	vuFromArc, err := runnerFromArc.NewVU(t.Context(), 2, 2, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	assert.Equal(t, 4, checkModule.initCtxCalled)
	assert.Equal(t, 2, checkModule.vuCtxCalled)
	activeVUFromArc := vuFromArc.Activate(&lib.VUActivationParams{RunContext: vuCtx})
	require.NoError(t, activeVUFromArc.RunOnce())
	assert.Equal(t, 4, checkModule.initCtxCalled)
	assert.Equal(t, 3, checkModule.vuCtxCalled)
	require.NoError(t, activeVUFromArc.RunOnce())
	assert.Equal(t, 4, checkModule.initCtxCalled)
	assert.Equal(t, 4, checkModule.vuCtxCalled)
}
