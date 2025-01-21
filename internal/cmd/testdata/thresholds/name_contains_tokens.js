//  The threshold name contains the '{' and '}' characters, which
// are used as tokens when parsing the submetric part of a threshold's
// name. This pattern occurs when following the URL grouping pattern, and
// should not error.");
//
// Protects from #2512 regressions.
export const options = {
	thresholds: {
		"http_req_duration{name:http://${}.com}": ["max < 1000"],
	},
};

export default function () {
	console.log(
		"asserting a threshold's name containing parsable tokens is valid"
	);
}
