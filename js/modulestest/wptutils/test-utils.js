// Original source file: https://github.com/web-platform-tests/wpt/blob/7eaf605c38d80377c717828376deabad86b702b2/streams/resources/test-utils.js
'use strict';

function delay(ms) {
	return new Promise(resolve => step_timeout(resolve, ms));
}

// For tests which verify that the implementation doesn't do something it shouldn't, it's better not to use a
// timeout. Instead, assume that any reasonable implementation is going to finish work after 2 times around the event
// loop, and use flushAsyncEvents().then(() => assert_array_equals(...));
// Some tests include promise resolutions which may mean the test code takes a couple of event loop visits itself. So go
// around an extra 2 times to avoid complicating those tests.
function flushAsyncEvents() {
	return delay(0)
		.then(() => delay(0))
		.then(() => delay(0))
		.then(() => delay(0));
}