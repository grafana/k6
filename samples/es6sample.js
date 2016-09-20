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

		group("json", function() {
			let res = http.get("https://httpbin.org/get", null, {
				headers: { "X-Test": "abc123" },
			});
			test(res, {
				"status is 200": (res) => res.status === 200,
				"X-Test header is correct": (res) => res.json().headers['X-Test'] === "abc123",
			});
			// console.log(res.body);
		});

		group("html", function() {
			test(http.get("http://test.loadimpact.com/"), {
				"status is 200": (res) => res.status === 200,
				"welcome message is correct": (res) => res.html("h2").text() === "Welcome to the LoadImpact.com demo site!",
			});
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
