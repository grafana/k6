package js

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
)

func BenchmarkHTTPRequests(b *testing.B) {
	b.StopTimer()
	tb := httpmultibin.NewHTTPMultiBin(b)
	defer tb.Cleanup()

	r, err := getSimpleRunner("/script.js", tb.Replacer.Replace(`
			import http from "k6/http";
			export default function() {
				let url = "HTTPBIN_URL";
				let res = http.get(url + "/cookies/set?k2=v2&k1=v1");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			}
		`))
	if !assert.NoError(b, err) {
		return
	}
	r.SetOptions(lib.Options{
		Throw:          null.BoolFrom(true),
		MaxRedirects:   null.IntFrom(10),
		Hosts:          tb.Dialer.Hosts,
		NoCookiesReset: null.BoolFrom(true),
	})

	var ch = make(chan stats.SampleContainer, 100)
	go func() { // read the channel so it doesn't block
		for {
			<-ch
		}
	}()
	initVU, err := r.NewVU(1, ch)
	if !assert.NoError(b, err) {
		return
	}
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: context.Background()})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err = vu.RunOnce()
		assert.NoError(b, err)
	}
}
