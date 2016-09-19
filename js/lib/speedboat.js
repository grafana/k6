export function group(name, fn, cond) {
	if (cond !== undefined && !cond) {
		return
	}

	return __jsapi__.DoGroup(name, fn);
}

export function test(name, ...sets) {
	return __jsapi__.DoTest(name, ...sets);
}

export function sleep(secs) {
	__jsapi__.Sleep(secs);
}

export default {
	group: group,
	test: test,
	sleep: sleep,
};
