// based on https://developer.mozilla.org/en-US/docs/Web/API/setTimeout#reasons_for_delays_longer_than_specified
import { setTimeout } from "k6/experimental/timers";
let last = 0;
let iterations = 10;

function timeout() {
	// log the time of this call
	logline(new Date().getMilliseconds());

	// if we are not finished, schedule the next call
	if (iterations-- > 0) {
		setTimeout(timeout, 0);
	}
}

export default function () {
	// initialize iteration count and the starting timestamp
	iterations = 10;
	last = new Date().getMilliseconds();

	// start timer
	setTimeout(timeout, 0);
}

function pad(number) {
	return number.toString().padStart(3, "0");
}

function logline(now) {
	// log the last timestamp, the new timestamp, and the difference
	console.log(`${pad(last)}         ${pad(now)}          ${now - last}`);
	last = now;
}
