package js

import (
	"context"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func BenchmarkHTTPRequests(b *testing.B) {
	b.StopTimer()
	tb := testutils.NewHTTPMultiBin(b)
	defer tb.Cleanup()

	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(tb.Replacer.Replace(`
			import http from "k6/http";
			export default function() {
				let url = "HTTPBIN_URL";
				let res = http.get(url + "/cookies/set?k2=v2&k1=v1");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			}
		`)),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(b, err) {
		return
	}
	r.SetOptions(lib.Options{
		Throw:        null.BoolFrom(true),
		MaxRedirects: null.IntFrom(10),
		Hosts:        tb.Dialer.Hosts,
	})

	var ch = make(chan stats.SampleContainer, 100)
	go func() { // read the channel so it doesn't block
		for {
			<-ch
		}
	}()
	vu, err := r.NewVU(ch)
	if !assert.NoError(b, err) {
		return
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err = vu.RunOnce(context.Background())
		assert.NoError(b, err)
	}
}
