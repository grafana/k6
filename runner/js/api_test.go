package js

import (
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"gopkg.in/olebedev/go-duktape.v2"
	"testing"
	"time"
)

func testJSContext() (*Runner, *duktape.Context, chan runner.Result) {
	ch := make(chan runner.Result, 100)
	r := New()
	c, err := r.newJSContext(loadtest.LoadTest{
		URL:    "http://example.com",
		Script: "script.js",
		Source: "~ not actually valid JS ~",
		Stages: []loadtest.Stage{
			loadtest.Stage{VUs: loadtest.VUSpec{Start: 10, End: 100}, Duration: 10 * time.Second},
		},
	}, 1, ch)
	if err != nil {
		panic(err)
	}
	return r, c, ch
}

func TestHTTPSetMaxConnectionsPerHost(t *testing.T) {
	src := `require('http').setMaxConnectionsPerHost(200);`
	r, c, _ := testJSContext()
	if err := c.PevalString(src); err != nil {
		t.Fatalf("Couldn't run script: %s", err)
	}
	if r.Client.MaxConnsPerHost != 200 {
		t.Fatalf("Incorrect number of max connections: %d", r.Client.MaxConnsPerHost)
	}
}

func TestHTTPSetMaxConnectionsPerHostNegative(t *testing.T) {
	src := `require('http').setMaxConnectionsPerHost(-1);`
	r, c, ch := testJSContext()
	before := r.Client.MaxConnsPerHost
	if err := c.PevalString(src); err != nil {
		t.Fatalf("Couldn't run script: %s", err)
	}
	select {
	case res := <-ch:
		if res.Error == nil {
			t.Error("No error reported!")
		}
		if r.Client.MaxConnsPerHost != before {
			t.Errorf("Max connections changed! %d", r.Client.MaxConnsPerHost)
		}
	default:
		t.Error("No results")
	}
}
