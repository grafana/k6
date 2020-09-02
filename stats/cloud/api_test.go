/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package cloud

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
)

func init() {
	_ = os.Setenv("K6CLOUD_HOST", "")
	_ = os.Setenv("K6CLOUD_TOKEN", "")
}

func fprintf(t *testing.T, w io.Writer, format string, a ...interface{}) int {
	n, err := fmt.Fprintf(w, format, a...)
	require.NoError(t, err)
	return n
}

func TestCreateTestRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fprintf(t, w, `{"reference_id": "1", "config": {"aggregationPeriod": "2s"}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")

	tr := &TestRun{
		Name: "test",
	}
	resp, err := client.CreateTestRun(tr)

	assert.Nil(t, err)
	assert.Equal(t, resp.ReferenceID, "1")
	assert.NotNil(t, resp.ConfigOverride)
	assert.True(t, resp.ConfigOverride.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(2*time.Second), resp.ConfigOverride.AggregationPeriod.Duration)
	assert.False(t, resp.ConfigOverride.AggregationMinSamples.Valid)
}

func TestPublishMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g, err := gzip.NewReader(r.Body)

		require.NoError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, g)
		require.NoError(t, err)
		byteCount, err := strconv.Atoi(r.Header.Get("x-payload-byte-count"))
		require.NoError(t, err)
		require.Equal(t, buf.Len(), byteCount)

		samplesCount, err := strconv.Atoi(r.Header.Get("x-payload-sample-count"))
		require.NoError(t, err)
		var samples []*Sample
		err = json.Unmarshal(buf.Bytes(), &samples)
		require.NoError(t, err)
		require.Equal(t, len(samples), samplesCount)

		fprintf(t, w, "")
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")

	samples := []*Sample{
		{
			Type:   "Point",
			Metric: "metric",
			Data: &SampleDataSingle{
				Type:  1,
				Time:  toMicroSecond(time.Now()),
				Value: 1.2,
			},
		},
	}
	err := client.PushMetric("1", false, samples)

	assert.Nil(t, err)
}

func TestFinished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fprintf(t, w, "")
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")

	thresholds := map[string]map[string]bool{
		"threshold": {
			"max < 10": true,
		},
	}
	err := client.TestFinished("1", thresholds, true, 0)

	assert.Nil(t, err)
}

func TestAuthorizedError(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
		fprintf(t, w, `{"error": {"code": 5, "message": "Not allowed"}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")

	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 1, called)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "(403/E5) Not allowed")
}

func TestDetailsError(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
		fprintf(t, w, `{"error": {"code": 0, "message": "Validation failed", "details": { "name": ["Shorter than minimum length 2."]}}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")

	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 1, called)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "(403) Validation failed\n name: Shorter than minimum length 2.")
}

func TestRetry(t *testing.T) {
	called := 0
	idempotencyKey := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
		if idempotencyKey == "" {
			idempotencyKey = gotK6IdempotencyKey
		}
		assert.NotEmpty(t, gotK6IdempotencyKey)
		assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
		called++
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 3, called)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestRetrySuccessOnSecond(t *testing.T) {
	called := 1
	idempotencyKey := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
		if idempotencyKey == "" {
			idempotencyKey = gotK6IdempotencyKey
		}
		assert.NotEmpty(t, gotK6IdempotencyKey)
		assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
		called++
		if called == 2 {
			fprintf(t, w, `{"reference_id": "1"}`)
			return
		}
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 2, called)
	assert.NotNil(t, resp)
	assert.Nil(t, err)
}

func TestIdempotencyKey(t *testing.T) {
	const idempotencyKey = "xxx"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
		switch r.Method {
		case http.MethodPost:
			assert.NotEmpty(t, gotK6IdempotencyKey)
			assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
		default:
			assert.Empty(t, gotK6IdempotencyKey)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0")
	client.retryInterval = 1 * time.Millisecond
	req, err := client.NewRequest(http.MethodPost, server.URL, nil)
	assert.NoError(t, err)
	req.Header.Set(k6IdempotencyKeyHeader, idempotencyKey)
	assert.NoError(t, client.Do(req, nil))

	req, err = client.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)
	assert.NoError(t, client.Do(req, nil))
}
