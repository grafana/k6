/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/minirunner"
	"github.com/loadimpact/k6/stats"

	"github.com/prometheus/common/expfmt"
)

func TestGetMonitor(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	execScheduler, err := local.NewExecutionScheduler(&minirunner.MiniRunner{}, logger)
	require.NoError(t, err)
	engine, err := core.NewEngine(execScheduler, lib.Options{}, logger)
	require.NoError(t, err)

	engine.Metrics = map[string]*stats.Metric{
		"my_trend": stats.New("my_trend", stats.Trend, stats.Time),
	}
	engine.Metrics["my_trend"].Tainted = null.BoolFrom(true)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/monitor", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", res.Header.Get("Content-Type"))

	t.Run("metrics", func(t *testing.T) {
		parser := expfmt.TextParser{}
		metrics, err := parser.TextToMetricFamilies(rw.Body)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Len(t, metrics, 6)
		suffixes := []string{"_min", "_max", "_avg", "_med", "_p90", "_p95"}
		for _, suffix := range suffixes {
			name := "my_trend" + suffix
			assert.Equal(t, name, metrics[name].GetName())
		}
	})
}
