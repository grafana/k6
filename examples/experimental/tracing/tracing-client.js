import http from "k6/http";
import { check } from "k6";
import tracing from "k6/experimental/tracing";

// Explicitly instantiating a tracing client allows to distringuish
// instrumented from non-instrumented HTTP calls, by keeping APIs separate.
// It also allows for finer-grained configuration control, by letting
// users changing the tracing configuration on the fly during their
// script's execution.
let instrumentedHTTP = new tracing.Client({
	propagator: "w3c",
});

const testData = { name: "Bert" };

export default () => {
	// Using the tracing client instance, HTTP calls will have
	// their trace context headers set.
	let res = instrumentedHTTP.request("GET", "http://httpbin.org/get", null, {
		headers: {
			"X-Example-Header": "instrumented/request",
		},
	});
	check(res, {
		"status is 200": (r) => r.status === 200,
	});

	// The tracing client offers more flexibility over
	// the `instrumentHTTP` function, as it leaves the
	// imported standard http module untouched. Thus,
	// one can still perform non-instrumented HTTP calls
	// using it.
	res = http.post("http://httpbin.org/post", JSON.stringify(testData), {
		headers: { "X-Example-Header": "noninstrumented/post" },
	});
	check(res, {
		"status is 200": (r) => r.status === 200,
	});

	res = instrumentedHTTP.del("http://httpbin.org/delete", null, {
		headers: { "X-Example-Header": "instrumented/delete" },
	});
	check(res, {
		"status is 200": (r) => r.status === 200,
	});
};
