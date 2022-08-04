package k6_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
)

func TestSetupDataMarshalling(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := js.New(
		&lib.TestPreInitState{
			Logger:         testutils.NewLogger(t),
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
		},

		&loader.SourceData{URL: &url.URL{Path: "/script.js"}, Data: script},
		nil,
	)

	require.NoError(t, err)

	err = runner.SetOptions(lib.Options{
		SetupTimeout: types.NullDurationFrom(5 * time.Second),
		Hosts:        tb.Dialer.Hosts,
	})

	require.NoError(t, err)

	samples := make(chan<- metrics.SampleContainer, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !assert.NoError(t, runner.Setup(ctx, samples)) {
		return
	}
	initVU, err := runner.NewVU(1, 1, samples)
	if assert.NoError(t, err) {
		vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
		err := vu.RunOnce()
		assert.NoError(t, err)
	}
}
