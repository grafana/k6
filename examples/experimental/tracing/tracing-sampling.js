import http from "k6/http";
import { check } from "k6";
import tracing from "k6/experimental/tracing";

export const options = {
	// As the number of sampled requests will converge towards
	// the sampling percentage, we need to increase the number
	// of iterations to get a more accurate result.
	iterations: 10000,

	vus: 100,
};

tracing.instrumentHTTP({
	propagator: "w3c",

	// Only 10% of the requests made will have their trace context
	// header's sample flag set to activated.
	sampling: 0.1,
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
