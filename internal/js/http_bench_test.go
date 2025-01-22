package js

import (
	"context"
	"testing"

	"go.k6.io/k6/lib/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func BenchmarkHTTPRequests(b *testing.B) {
	b.StopTimer()
	tb := httpmultibin.NewHTTPMultiBin(b)

	r, err := getSimpleRunner(b, "/script.js", tb.Replacer.Replace(`
			import http from "k6/http";
			export default function() {
				let url = "HTTPBIN_URL";
				let res = http.get(url + "/cookies/set?k2=v2&k1=v1");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			}
		`), lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")})
	require.NoError(b, err)

	err = r.SetOptions(lib.Options{
		Throw:          null.BoolFrom(true),
		MaxRedirects:   null.IntFrom(10),
		Hosts:          types.NullHosts{Trie: tb.Dialer.Hosts},
		NoCookiesReset: null.BoolFrom(true),
		SystemTags:     &metrics.DefaultSystemTagSet,
		RunTags:        map[string]string{"myapp": "myhttpbench"},
	})
	require.NoError(b, err)

	ch := newDevNullSampleChannel()
	defer close(ch)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initVU, err := r.NewVU(ctx, 1, 1, ch)
	require.NoError(b, err)
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, vu.RunOnce())
	}
}

func BenchmarkHTTPRequestsBase(b *testing.B) {
	b.StopTimer()
	tb := httpmultibin.NewHTTPMultiBin(b)

	r, err := getSimpleRunner(b, "/script.js", tb.Replacer.Replace(`
			var http = require("k6/http");
			exports.default = function() {
				var url = "HTTPBIN_URL";
				var res = http.get(url + "/cookies/set?k2=v2&k1=v1");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			}
		`))
	require.NoError(b, err)
	err = r.SetOptions(lib.Options{
		Throw:          null.BoolFrom(true),
		MaxRedirects:   null.IntFrom(10),
		Hosts:          types.NullHosts{Trie: tb.Dialer.Hosts},
		NoCookiesReset: null.BoolFrom(true),
	})
	require.NoError(b, err)

	ch := newDevNullSampleChannel()
	defer close(ch)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initVU, err := r.NewVU(ctx, 1, 1, ch)
	require.NoError(b, err)
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, vu.RunOnce())
	}
}
