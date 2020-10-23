import { group, check, sleep } from "k6";
import { Counter, Rate } from "k6/metrics";
import http from "k6/http";

export let options = {
	vus: 5,
	thresholds: {
		my_rate: ["rate>=0.4"], // Require my_rate's success rate to be >=40%
		http_req_duration: ["avg<1000"], // Require http_req_duration's average to be <1000ms
	}
};

let mCounter = new Counter("my_counter");
let mRate = new Rate("my_rate");

export default function() {
	check(Math.random(), {
		"top-level test": (v) => v < 1/3
	});
	group("my group", function() {
		mCounter.add(1, { tag: "test" });

		check(Math.random(), {
			"random value is < 0.5": (v) => mRate.add(v < 0.5),
		});

		group("json", function() {
			let res = http.get("https://httpbin.org/get", {
				headers: { "X-Test": "abc123" },
			});

			check(res, {
				"status is 200": (res) => res.status === 200,
				"X-Test header is correct": (res) => res.json().headers['X-Test'] === "abc123",
			});
		});

		group("html", function() {
			check(http.get("http://test.k6.io/"), {
				"status is 200": (res) => res.status === 200,
				"content type is html": (res) => res.headers['Content-Type'].startsWith("text/html"),
				"welcome message is correct": (res) => res.html("p.description").text() === "Collection of simple web-pages suitable for load testing.",
			});
		});

		group("nested", function() {
			check(null, {
				"always passes": true,
				"always fails": false,
			});
		});
	});
	sleep(10 * Math.random());
};
