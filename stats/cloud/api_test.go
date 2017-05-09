package cloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateTestRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"reference_id": "1"}`)
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	tr := &TestRun{
		Name: "test",
	}
	resp, err := client.CreateTestRun(tr)

	assert.Nil(t, err)
	assert.Equal(t, resp.ReferenceID, "1")
}

func TestPublishMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	samples := []*sample{
		{
			Type:   "Point",
			Metric: "metric",
			Data: sampleData{
				Type:  1,
				Time:  time.Now(),
				Value: 1.2,
			},
		},
	}
	err := client.PushMetric("1", samples)

	assert.Nil(t, err)
}

func TestFinished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	thresholds := map[string]map[string]bool{
		"threshold": {
			"max < 10": true,
		},
	}
	err := client.TestFinished("1", thresholds, true)

	assert.Nil(t, err)
}
