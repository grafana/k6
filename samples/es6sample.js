import { group, test, sleep } from "speedboat";

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

		group("nested", function() {
			test(null, {
				"always passes": true,
				"always fails": false,
			});
		});
	});
	sleep(10 * Math.random());
};
