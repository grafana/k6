import http from "k6/http";
import { check } from "k6";
import tracing from "k6/experimental/tracing";

// instrumentHTTP will ensure that all requests made by the http module
// will be traced. The first argument is a configuration object that
// can be used to configure the tracer.
//
// Currently supported HTTP methods are: get, post, put, patch, head,
// del, options, and request.
tracing.instrumentHTTP({
	propagator: "w3c",
});

export default () => {
	let res = http.get("http://httpbin.org/get", {
		headers: {
			"X-Example-Header": "instrumented/get",
		},
	});
	check(res, {
		"status is 200": (r) => r.status === 200,
	});
};
