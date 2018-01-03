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

package http

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/oxtoacart/bpool"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v3"
)

func TestResponse(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	logger := log.New()
	logger.Level = log.DebugLevel
	logger.Out = ioutil.Discard

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	state := &common.State{
		Options: lib.Options{
			MaxRedirects: null.IntFrom(10),
			UserAgent:    null.StringFrom("TestUserAgent"),
			Throw:        null.BoolFrom(true),
		},
		Logger: logger,
		Group:  root,
		HTTPTransport: &http.Transport{
			DialContext: (netext.NewDialer(net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			})).DialContext,
		},
		BPool: bpool.NewBufferPool(1),
	}

	ctx := new(context.Context)
	*ctx = context.Background()
	*ctx = common.WithState(*ctx, state)
	*ctx = common.WithRuntime(*ctx, rt)
	rt.Set("http", common.Bind(rt, New(), ctx))

	t.Run("Html", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://httpbin.org/html");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/html", "", 200, "")

		t.Run("html", func(t *testing.T) {
			_, err := common.RunString(rt, `
			if (res.html().find("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
			`)
			assert.NoError(t, err)

			t.Run("shorthand", func(t *testing.T) {
				_, err := common.RunString(rt, `
				if (res.html("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
				`)
				assert.NoError(t, err)
			})

			t.Run("url", func(t *testing.T) {
				_, err := common.RunString(rt, `
				if (res.html().url != "https://httpbin.org/html") { throw new Error("url incorrect: " + res.html().url); }
				`)
				assert.NoError(t, err)
			})
		})

		t.Run("group", func(t *testing.T) {
			g, err := root.Group("my group")
			if assert.NoError(t, err) {
				old := state.Group
				state.Group = g
				defer func() { state.Group = old }()
			}

			state.Samples = nil
			_, err = common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/html");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/html", "", 200, "::my group")
		})
	})
	t.Run("Json", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://httpbin.org/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get?a=1&b=2", "", 200, "")

		t.Run("Invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `http.request("GET", "https://httpbin.org/html").json();`)
			assert.EqualError(t, err, "GoError: invalid character '<' looking for beginning of value")
		})
	})

	t.Run("SubmitForm", func(t *testing.T) {
		t.Run("withoutArgs", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/forms/post");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			res = res.submitForm()
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			let data = res.json().form
			if (data.custname !== "" ||
				data.extradata !== undefined || 
				data.comments !== "" || 
				data.custemail !== "" || 
				data.custtel !== "" || 
				data.delivery !== "" 
			) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
		`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "POST", "https://httpbin.org/post", "", 200, "")
		})

		t.Run("withFields", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/forms/post");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			res = res.submitForm({ fields: { custname: "test", extradata: "test2" } })
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			let data = res.json().form
			if (data.custname !== "test" ||
				data.extradata !== "test2" || 
				data.comments !== "" || 
				data.custemail !== "" || 
				data.custtel !== "" || 
				data.delivery !== "" 
			) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
		`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "POST", "https://httpbin.org/post", "", 200, "")
		})

		t.Run("withRequestParams", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/forms/post");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			res = res.submitForm({ params: { headers: { "My-Fancy-Header": "SomeValue" } }})
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			let headers = res.json().headers
			if (headers["My-Fancy-Header"] !== "SomeValue" ) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
		`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "POST", "https://httpbin.org/post", "", 200, "")
		})

		t.Run("withNonExistentForm", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/forms/post");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			res.submitForm({ formSelector: "#doesNotExist" })
		`)
			assert.EqualError(t, err, "GoError: no form found for selector '#doesNotExist' in response 'https://httpbin.org/forms/post'")
		})
	})
}
