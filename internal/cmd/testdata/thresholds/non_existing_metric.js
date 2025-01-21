export const options = {
	thresholds: {
		// non existing is neither registered, nor a builtin metric.
		// k6 should catch that.
		"non existing": ["rate>0"],
	},
};

export default function () {
	console.log(
		"asserting that a threshold over a non-existing metric fails with exit code 104 (Invalid config)"
	);
}
