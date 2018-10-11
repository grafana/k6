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

package k6_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func TestSetupDataMarshalling(t *testing.T) {
	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import html from "k6/html";
		import ws from "k6/ws";
		export let options = { setupTimeout: "10s" };


		function make_jar() {
			let jar = http.cookieJar();
			jar.set("test", "something")
			return jar;
		}

		export function setup() {
			let res = http.get("http://test.loadimpact.com:HTTPBIN_PORT");
			let html_selection = html.parseHTML(res.body); 
			let ws_res = ws.connect("ws://test.loadimpact.com:HTTPBIN_PORT/ws-echo", function(socket){
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
			return first.every(element => second.includes(element))
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
			let first_properties = get_non_function_properties(data).sort();
			diff_object_properties("setupdata", data, setup());
		}
	`))

	runner, err := js.New(
		&lib.SourceData{Filename: "/script.js", Data: script},
		afero.NewMemMapFs(),
		lib.RuntimeOptions{Env: map[string]string{"setupTimeout": "10s"}},
	)

	runner.SetOptions(lib.Options{
		SetupTimeout: types.NullDurationFrom(1 * time.Second),
		MaxRedirects: null.IntFrom(10),
		Throw:        null.BoolFrom(true),
		Hosts: map[string]net.IP{
			"test.loadimpact.com": net.ParseIP("127.0.0.1"),
		},
	})

	require.NoError(t, err)

	samples := make(chan stats.SampleContainer, 100)

	if !assert.NoError(t, runner.Setup(context.Background(), samples)) {
		return
	}
	vu, err := runner.NewVU(samples)
	if assert.NoError(t, err) {
		err := vu.RunOnce(context.Background())
		assert.NoError(t, err)
	}
}
