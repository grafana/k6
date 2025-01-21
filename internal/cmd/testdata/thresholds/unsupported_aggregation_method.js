export const options = {
	thresholds: {
		// http_reqs is a Counter metric. As such, it supports
		// only the 'count' and 'rate' operations. Thus, 'value'
		// being a Gauge's metric aggregation method, the threshold
		// configuration evaluation should fail.
		http_reqs: ["value>0"],
	},
};

export default function () {
	console.log(
		"asserting that a threshold applying a method over a metric not supporting it fails with exit code 104 (Invalid config)"
	);
}
