// In scenarios where a threshold would apply to a rate metric
// that would not receive any samples (settting abortToFail emphasis the issue),
// division by zero could occur and lead to NaN values being returned.
//
// Protects from #2520 regressions.
import { Rate } from "k6/metrics";

const rate = new Rate("rate");

export const options = {
	thresholds: {
		"rate{type:read}": [{ threshold: "rate>0.9", abortOnFail: true }],
	},
};

export default function () {
	console.log("not interacting with rate metric");
}
