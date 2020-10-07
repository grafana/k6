/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

package k6_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

func TestSetupDataMarshalling(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import html from "k6/html";
		import ws from "k6/ws";

		function make_jar() {
			let jar = http.cookieJar();
			jar.set("test", "something")
			return jar;
		}

		export function setup() {
			let res = http.get("HTTPBIN_URL/html");
			let html_selection = html.parseHTML(res.body);
			let ws_res = ws.connect("WSBIN_URL/ws-echo", function(socket){
				socket.on("open", function() {
					socket.send("test")
				})
				socket.on("message", function (data){
					if (!data=="test") {
						throw new Error ("echo'd data doesn't match our message!");
					}
					socket.close()
				});
			});

			return {
				http_response: res,
				html_selection: html_selection,
				html_element: html_selection.find('html'),
				html_attribute: html_selection.find('html').children('body').get(0).attributes(),
				jar: make_jar(),
				ws_res: ws_res,
			};
		}

		function get_non_function_properties(object) {
			return Object.keys(object).filter(element =>
				typeof(object[element]) !== "function");
		}

		function arrays_are_equal(first, second) {
			if (first.length != second.length) {
				return false
			}
			return first.every(function(element, idx) {
				return element === second[idx]
			});
		}

		function diff_object_properties(name, first, second) {
			let first_properties = get_non_function_properties(first).sort();
			let second_properties = get_non_function_properties(second).sort();
			if (!(arrays_are_equal(first_properties, second_properties))) {
				console.error("for " + name + ":\n" +
					"first_properties : " + JSON.stringify(first_properties) + "\n" +
					"second_properties: " + JSON.stringify(second_properties) + "\n" +
					"are not the same");
				throw new Error("not matching " + name);
			}
			first_properties.
				filter(element => typeof(first[element]) === "object").
					forEach(function(element) {
						diff_object_properties(name+"."+element,
											   first[element],
											   second[element]);
			});
		}

		export default function (data) {
			diff_object_properties("setupdata", data, setup());
		}
	`))

	runner, err := js.New(
		testutils.NewLogger(t),
		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
		lib.RuntimeOptions{},
	)

	require.NoError(t, err)

	err = runner.SetOptions(lib.Options{
		SetupTimeout: types.NullDurationFrom(5 * time.Second),
		Hosts:        tb.Dialer.Hosts,
	})

	require.NoError(t, err)

	samples := make(chan<- stats.SampleContainer, 100)

	if !assert.NoError(t, runner.Setup(context.Background(), samples)) {
		return
	}
	initVU, err := runner.NewVU(1, samples)
	if assert.NoError(t, err) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
		err := vu.RunOnce()
		assert.NoError(t, err)
	}
}
