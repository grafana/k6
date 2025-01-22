export const options = {
	thresholds: {
		http_reqs: ["foo&0"], // Counter
	},
};

export default function () {
	console.log(
		"asserting that a malformed threshold fails with exit code 104 (Invalid config)"
	);
}
