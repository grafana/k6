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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/manyminds/api2go/jsonapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/k6io/k6/core"
	"github.com/k6io/k6/core/local"
	"github.com/k6io/k6/lib"
	"github.com/k6io/k6/lib/testutils"
	"github.com/k6io/k6/lib/testutils/minirunner"
)

func TestGetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	execScheduler, err := local.NewExecutionScheduler(&minirunner.MiniRunner{}, logger)
	require.NoError(t, err)
	engine, err := core.NewEngine(execScheduler, lib.Options{}, lib.RuntimeOptions{}, nil, logger)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/status", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		var doc jsonapi.Document
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		if !assert.NotNil(t, doc.Data.DataObject) {
			return
		}
		assert.Equal(t, "status", doc.Data.DataObject.Type)
	})

	t.Run("status", func(t *testing.T) {
		var status Status
		assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &status))
		assert.True(t, status.Paused.Valid)
		assert.True(t, status.VUs.Valid)
		assert.True(t, status.VUsMax.Valid)
		assert.False(t, status.Stopped)
		assert.False(t, status.Tainted)
	})
}

func TestPatchStatus(t *testing.T) {
	testdata := map[string]struct {
		StatusCode int
		Status     Status
	}{
		"nothing":               {200, Status{}},
		"paused":                {200, Status{Paused: null.BoolFrom(true)}},
		"max vus":               {200, Status{VUsMax: null.IntFrom(20)}},
		"max vus below initial": {400, Status{VUsMax: null.IntFrom(5)}},
		"too many vus":          {400, Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(0)}},
		"vus":                   {200, Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(10)}},
	}
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))

	scenarios := lib.ScenarioConfigs{}
	err := json.Unmarshal([]byte(`
			{"external": {"executor": "externally-controlled",
			"vus": 0, "maxVUs": 10, "duration": "1s"}}`), &scenarios)
	require.NoError(t, err)
	options := lib.Options{Scenarios: scenarios}

	for name, indata := range testdata {
		t.Run(name, func(t *testing.T) {
			execScheduler, err := local.NewExecutionScheduler(&minirunner.MiniRunner{Options: options}, logger)
			require.NoError(t, err)
			engine, err := core.NewEngine(execScheduler, options, lib.RuntimeOptions{}, nil, logger)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			run, _, err := engine.Init(ctx, ctx)
			require.NoError(t, err)

			go func() { _ = run() }()
			// wait for the executor to initialize to avoid a potential data race below
			time.Sleep(100 * time.Millisecond)

			body, err := jsonapi.Marshal(indata.Status)
			if !assert.NoError(t, err) {
				return
			}

			rw := httptest.NewRecorder()
			NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "PATCH", "/v1/status", bytes.NewReader(body)))
			res := rw.Result()

			if !assert.Equal(t, indata.StatusCode, res.StatusCode) {
				return
			}
			if indata.StatusCode != 200 {
				return
			}

			status := NewStatus(engine)
			if indata.Status.Paused.Valid {
				assert.Equal(t, indata.Status.Paused, status.Paused)
			}
			if indata.Status.VUs.Valid {
				assert.Equal(t, indata.Status.VUs, status.VUs)
			}
			if indata.Status.VUsMax.Valid {
				assert.Equal(t, indata.Status.VUsMax, status.VUsMax)
			}
		})
	}
}
