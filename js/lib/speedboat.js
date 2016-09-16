export function group(name, fn, cond) {
	if (cond !== undefined && !cond) {
		return
	}

	return __vu_impl__.DoGroup(name, fn);
}

export function test(name, ...sets) {
	return __vu_impl__.DoTest(name, ...sets);
}

export function sleep(secs) {
	__vu_impl__.Sleep(secs);
}
