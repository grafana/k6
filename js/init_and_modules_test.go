/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/internal/modules"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

type CheckModule struct {
	t             testing.TB
	initCtxCalled int
	vuCtxCalled   int
}

func (cm *CheckModule) InitCtx(ctx context.Context) {
	cm.initCtxCalled++
	assert.NotNil(cm.t, common.GetRuntime(ctx))
	assert.NotNil(cm.t, common.GetInitEnv(ctx))
	assert.Nil(cm.t, lib.GetState(ctx))
}

func (cm *CheckModule) VuCtx(ctx context.Context) {
	cm.vuCtxCalled++
	assert.NotNil(cm.t, common.GetRuntime(ctx))
	assert.Nil(cm.t, common.GetInitEnv(ctx))
	assert.NotNil(cm.t, lib.GetState(ctx))
}

func TestNewJSRunnerWithCustomModule(t *testing.T) {
	checkModule := &CheckModule{t: t}
	modules.Register("k6/check", checkModule)

	script := `
		var check = require("k6/check");
		check.initCtx();

		module.exports.options = { vus: 1, iterations: 1 };
		module.exports.default = function() {
			check.vuCtx();
		};
	`

	logger := testutils.NewLogger(t)
	rtOptions := lib.RuntimeOptions{CompatibilityMode: null.StringFrom("base")}
	runner, err := js.New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: "blah", Scheme: "file"},
			Data: []byte(script),
		},
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": afero.NewMemMapFs()},
		rtOptions,
	)
	require.NoError(t, err)
	assert.Equal(t, checkModule.initCtxCalled, 1)
	assert.Equal(t, checkModule.vuCtxCalled, 0)

	vu, err := runner.NewVU(1, make(chan stats.SampleContainer, 100))
	require.NoError(t, err)
	assert.Equal(t, checkModule.initCtxCalled, 2)
	assert.Equal(t, checkModule.vuCtxCalled, 0)

	vuCtx, vuCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer vuCancel()

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: vuCtx})
	require.NoError(t, activeVU.RunOnce())
	assert.Equal(t, checkModule.initCtxCalled, 2)
	assert.Equal(t, checkModule.vuCtxCalled, 1)
	require.NoError(t, activeVU.RunOnce())
	assert.Equal(t, checkModule.initCtxCalled, 2)
	assert.Equal(t, checkModule.vuCtxCalled, 2)

	arc := runner.MakeArchive()
	assert.Equal(t, checkModule.initCtxCalled, 2) // shouldn't change, we're not executing the init context again
	assert.Equal(t, checkModule.vuCtxCalled, 2)

	runnerFromArc, err := js.NewFromArchive(logger, arc, rtOptions)
	require.NoError(t, err)
	assert.Equal(t, checkModule.initCtxCalled, 3) // changes because we need to get the exported functions
	assert.Equal(t, checkModule.vuCtxCalled, 2)
	vuFromArc, err := runnerFromArc.NewVU(2, make(chan stats.SampleContainer, 100))
	require.NoError(t, err)
	assert.Equal(t, checkModule.initCtxCalled, 4)
	assert.Equal(t, checkModule.vuCtxCalled, 2)
	activeVUFromArc := vuFromArc.Activate(&lib.VUActivationParams{RunContext: vuCtx})
	require.NoError(t, activeVUFromArc.RunOnce())
	assert.Equal(t, checkModule.initCtxCalled, 4)
	assert.Equal(t, checkModule.vuCtxCalled, 3)
	require.NoError(t, activeVUFromArc.RunOnce())
	assert.Equal(t, checkModule.initCtxCalled, 4)
	assert.Equal(t, checkModule.vuCtxCalled, 4)
}
