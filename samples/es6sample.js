import { group, test, sleep } from "speedboat";
import http from "speedboat/http";

export let options = {
	vus: 5,
};

export default function() {
	test(Math.random(), {
		"top-level test": (v) => v < 1/3
	});
	group("my group", function() {
		test(Math.random(), {
			"random value is < 0.5": (v) => v < 0.5
		});

		group("http", function() {
			let res = http.get("https://httpbin.org/get", null, {
				headers: { "X-Test": "abc123" },
			});
			test(res, {
				"status is 200": (res) => res.status === 200,
				"X-Test header is correct": (res) => res.json().headers['X-Test'] === "abc123",
			});
			// console.log(res.body);
		});

		group("nested", function() {
			test(null, {
				"always passes": true,
				"always fails": false,
			});
		});
	});
	sleep(10 * Math.random());
};
