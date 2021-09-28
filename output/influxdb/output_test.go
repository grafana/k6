/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package influxdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	influxdomain "github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("BucketRequired", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: "/",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Bucket value is required")
	})
	t.Run("ForceDatabaseAsBucket", func(t *testing.T) {
		t.Parallel()
		o, err := New(output.Params{
			Logger:     testutils.NewLogger(t),
			JSONConfig: []byte(`{"db":"dbtest"}`),
		})
		require.NoError(t, err)
		assert.Equal(t, "dbtest", o.(*Output).Config.Bucket.String)
	})
	t.Run("ForceUserPasswordAsToken", func(t *testing.T) {
		t.Parallel()
		o, err := New(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: "http://myuser:mypassword@addr/bucketname",
		})
		require.NoError(t, err)
		assert.Equal(t, "myuser:mypassword", o.(*Output).Config.Token.String)
	})
	t.Run("BadConcurrentWrites", func(t *testing.T) {
		t.Parallel()
		logger := testutils.NewLogger(t)
		t.Run("0", func(t *testing.T) {
			t.Parallel()
			_, err := New(output.Params{
				Logger:         logger,
				ConfigArgument: "/bucketname?concurrentWrites=0",
			})
			require.Error(t, err)
			require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
		})

		t.Run("-2", func(t *testing.T) {
			t.Parallel()
			_, err := New(output.Params{
				Logger:         logger,
				ConfigArgument: "/bucketname?concurrentWrites=-2",
			})
			require.Error(t, err)
			require.Equal(t, "influxdb's ConcurrentWrites must be a positive number", err.Error())
		})

		t.Run("2", func(t *testing.T) {
			t.Parallel()
			_, err := New(output.Params{
				Logger:         logger,
				ConfigArgument: "/bucketname?concurrentWrites=2",
			})
			require.NoError(t, err)
		})
	})
}

func TestExtractTagsToValues(t *testing.T) {
	t.Parallel()
	o, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		JSONConfig:     []byte(`{"bucket":"mybucket"}`),
		ConfigArgument: "?tagsAsFields=stringField&tagsAsFields=stringField2:string&tagsAsFields=boolField:bool&tagsAsFields=floatField:float&tagsAsFields=intField:int",
	})
	require.NoError(t, err)
	tags := map[string]string{
		"stringField":  "string",
		"stringField2": "string2",
		"boolField":    "true",
		"floatField":   "3.14",
		"intField":     "12345",
	}
	values := o.extractTagsToValues(tags, map[string]interface{}{})

	require.Equal(t, "string", values["stringField"])
	require.Equal(t, "string2", values["stringField2"])
	require.Equal(t, true, values["boolField"])
	require.Equal(t, 3.14, values["floatField"])
	require.Equal(t, int64(12345), values["intField"])
}

func TestOutput(t *testing.T) {
	t.Parallel()
	var samplesRead int
	defer func() {
		require.Equal(t, 20, samplesRead)
	}()
	testOutputCycle(t, func(rw http.ResponseWriter, r *http.Request) {
		// on startup the version returned from the server is checked
		if r.URL.Path == "/health" {
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"status":"pass","version":"2.0"}`))
			return
		}
		// on startup if bucket exists is checked
		if r.URL.Path == "/api/v2/buckets" {
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"buckets":[{"name":"mybucket"}]}`))
			return
		}
		b := bytes.NewBuffer(nil)
		_, _ = io.Copy(b, r.Body)
		for {
			s, err := b.ReadString('\n')
			if len(s) > 0 {
				samplesRead++
			}
			if err != nil {
				break
			}
		}

		rw.WriteHeader(204)
	}, func(tb testing.TB, c *Output) {
		samples := make(stats.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = stats.Sample{
				Metric: stats.New("testGauge", stats.Gauge),
				Time:   time.Now(),
				Tags: stats.NewSampleTags(map[string]string{
					"something": "else",
					"VU":        "21",
					"else":      "something",
				}),
				Value: 2.0,
			}
		}
		c.AddMetricSamples([]stats.SampleContainer{samples})
		c.AddMetricSamples([]stats.SampleContainer{samples})
	})
}

func TestOutputStart(t *testing.T) {
	t.Parallel()

	t.Run("SuccessWithBucketCreation", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{Orgs: []string{"org1"}}
		ts := startInfluxv2Mock(&dataset)
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"bucket":"mybucket","organization":"org1"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
		assert.Contains(t, dataset.Buckets, "mybucket")
	})
	t.Run("SuccessWithAlreadyExistingBucket", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{
			Orgs:    []string{"org1"},
			Buckets: []string{"bucket1"},
		}
		ts := startInfluxv2Mock(&dataset)
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"bucket":"mybucket","organization":"org1"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
	})
	t.Run("SuccessWithDatabase18Creation", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{}
		ts := startInfluxv1Mock(&dataset, "", "")
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"db":"mydb"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
		assert.Contains(t, dataset.Databases, "mydb")
	})
	t.Run("SuccessWithAlreadyExistingDatabase18", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{Databases: []string{"db1"}}
		ts := startInfluxv1Mock(&dataset, "", "")
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"db":"db1"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
	})
	t.Run("SuccessWithDatabaseAuth", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{Databases: []string{"db1"}}
		ts := startInfluxv1Mock(&dataset, "joe", "passw")
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"db":"db1","username":"joe","password":"passw"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
	})
	t.Run("SkipCreationOnHealthcheckFail", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{Databases: []string{"db1"}}
		ts := startTelegrafMock(&dataset)
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"bucket":"mybucket","organization":"org1"}`),
		})
		require.NoError(t, err)
		require.NoError(t, c.Start())
		assert.Len(t, dataset.Databases, 1)
	})
	t.Run("DatabaseAuthFailed", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{Databases: []string{"db1"}}
		ts := startInfluxv1Mock(&dataset, "joe", "passw")
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"db":"db1","username":"joe","password":"p"}`),
		})
		require.NoError(t, err)
		assert.NotNil(t, c.Start())
	})
	t.Run("OrganizationDoesNotExist", func(t *testing.T) {
		t.Parallel()

		dataset := influxMemory{
			Orgs:    []string{"org1"},
			Buckets: []string{"bucket1"},
		}
		ts := startInfluxv2Mock(&dataset)
		defer ts.Close()

		c, err := newOutput(output.Params{
			Logger:         testutils.NewLogger(t),
			ConfigArgument: ts.URL,
			JSONConfig:     []byte(`{"bucket":"mybucket","organization":"org0"}`),
		})
		require.NoError(t, err)
		err = c.Start()
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "organization 'org0' not found")
	})
}

type influxMemory struct {
	Orgs    []string
	Buckets []string

	// InfluxDB 1.x
	Databases []string
}

func (db *influxMemory) FilterBuckets(name string) []string {
	items := make([]string, 0, len(db.Buckets))
	for _, bucket := range db.Buckets {
		// apply the filter if required
		if name != "" && name != bucket {
			continue
		}
		items = append(items, fmt.Sprintf(`{"name":%q}`, bucket))
	}
	return items
}

func (db *influxMemory) FilterOrgs(name string) []string {
	items := make([]string, 0, len(db.Orgs))
	for i, org := range db.Orgs {
		if name != "" && name != org {
			continue
		}
		items = append(items, fmt.Sprintf(`{"id":"%d","name":%q}`, i, org))
	}
	return items
}

func testOutputCycle(t testing.TB, handler http.HandlerFunc, body func(testing.TB, *Output)) {
	s := &http.Server{
		Addr:           ":",
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = l.Close()
	}()

	defer func() {
		require.NoError(t, s.Shutdown(context.Background()))
	}()

	go func() {
		require.Equal(t, http.ErrServerClosed, s.Serve(l))
	}()
	c, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: "http://" + l.Addr().String(),
		JSONConfig:     []byte(`{"bucket":"mybucket"}`),
	})
	require.NoError(t, err)

	require.NoError(t, c.Start())
	body(t, c)

	require.NoError(t, c.Stop())
}

func startInfluxv1Mock(db *influxMemory, expuser, exppassword string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// on startup the version returned from the server is checked
		if r.URL.Path == "/health" {
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"status":"pass","version":"1.8"}`))
			return
		}

		if expuser != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != expuser || p != exppassword {
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		// create a database for version 1.x
		if r.URL.Path == "/query" {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			q := string(b)
			prefix := "q=CREATE+DATABASE+"
			if !strings.HasPrefix(q, prefix) {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			dbname := strings.TrimPrefix(q, prefix)
			db.Databases = append(db.Databases, dbname)
			return
		}

		rw.WriteHeader(http.StatusInternalServerError)
	}))
}

func startInfluxv2Mock(db *influxMemory) *httptest.Server {
	fn := func(rw http.ResponseWriter, r *http.Request) {
		// on startup the version returned from the server is checked
		if r.URL.Path == "/health" {
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"status":"pass","version":"2.0"}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/buckets" {
			// list buckets
			filter := r.URL.Query().Get("name")
			buckets := strings.Join(db.FilterBuckets(filter), ",")

			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(fmt.Sprintf(`{"buckets":[%s]}`, buckets)))
			return
		}

		// create a bucket
		if r.Method == http.MethodPost && r.URL.Path == "/api/v2/buckets" {
			var bucket influxdomain.Bucket
			err := json.NewDecoder(r.Body).Decode(&bucket)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			db.Buckets = append(db.Buckets, bucket.Name)
			return
		}

		// list orgs
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/orgs" {
			filter := r.URL.Query().Get("org")
			orgs := strings.Join(db.FilterOrgs(filter), filter)

			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(fmt.Sprintf(`{"orgs":[%s]}`, orgs)))
			return
		}

		rw.WriteHeader(http.StatusInternalServerError)
	}
	return httptest.NewServer(http.HandlerFunc(fn))
}

func startTelegrafMock(_ *influxMemory) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
}
