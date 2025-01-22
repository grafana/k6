// Thresholds over submetrics without any values should still
// be displayed under their proper parent metrics in the summary.
//
// Protects from #2518 regressions.
import { Counter } from "k6/metrics";

const counter1 = new Counter("one");
const counter2 = new Counter("two");

export const options = {
	thresholds: {
		"one{tag:xyz}": [],
	},
};

export default function () {
	console.log("not submitting metric1");
	counter2.add(42);
}
