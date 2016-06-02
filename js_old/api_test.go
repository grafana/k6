package js

import (
	"github.com/loadimpact/speedboat"
	"gopkg.in/olebedev/go-duktape.v2"
	"testing"
	"time"
)

func testJSContext() (*Runner, *duktape.Context) {
	r := New("")
	c, err := r.newJSContext(speedboat.Test{
		URL:    "http://example.com",
		Script: "script.js",
		Stages: []speedboat.TestStage{
			speedboat.TestStage{StartVUs: 10, EndVUs: 100, Duration: 10 * time.Second},
		},
	}, 1)
	if err != nil {
		panic(err)
	}
	return r, c
}

func TestHTTPSetMaxConnectionsPerHost(t *testing.T) {
	src := `require('http').setMaxConnectionsPerHost(200);`
	r, c := testJSContext()
	if err := c.PevalString(src); err != nil {
		t.Fatalf("Couldn't run script: %s", err)
	}
	if r.Client.MaxConnsPerHost != 200 {
		t.Fatalf("Incorrect number of max connections: %d", r.Client.MaxConnsPerHost)
	}
}

func TestHTTPSetMaxConnectionsPerHostNegative(t *testing.T) {
	src := `require('http').setMaxConnectionsPerHost(-1);`
	r, c := testJSContext()
	before := r.Client.MaxConnsPerHost
	if err := c.PevalString(src); err != nil {
		t.Fatalf("Couldn't run script: %s", err)
	}
	if r.Client.MaxConnsPerHost != before {
		t.Errorf("Max connections changed! %d", r.Client.MaxConnsPerHost)
	}
}
