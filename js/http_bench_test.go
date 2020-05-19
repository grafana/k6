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

package js

import (
	"context"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
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
