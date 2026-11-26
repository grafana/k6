import events from 'k6/x/events';

export const options = {
	vus: 3,
	iterations: 6,
};

export default function() {
	// No code needed here to trigger IterStart/IterEnd: k6 emits those
	// automatically around every call to this function, for every VU that
	// imported k6/x/events above.
}

export function teardown() {
	// Init, TestStart, IterStart and IterEnd will already have fired by now;
	// TestEnd and Exit fire after teardown returns, so they won't show up
	// here (see README for the full expected sequence).
	const counts = events.counts();

	console.log(`k6/x/events counts so far: ${JSON.stringify(counts)}`);

	if (!counts.Init || !counts.TestStart || !counts.IterStart || !counts.IterEnd) {
		throw new Error(
			`expected Init, TestStart, IterStart and IterEnd to have fired at least once, got: ${JSON.stringify(counts)}`
		);
	}

	if (counts.IterStart !== 6 || counts.IterEnd !== 6) {
		throw new Error(`expected 6 IterStart/IterEnd (one per iteration), got: ${JSON.stringify(counts)}`);
	}
}
