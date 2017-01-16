/**
 * @module k6
 */

/**
 * Runs code in a group.
 * @param  {string}   name   Name of the group.
 * @param  {Function} fn     Group body.
 * @param  {any}      [cond] If given, the group will be skipped if falsy.
 * @return {any}             The return value of fn().
 */
export function group(name, fn, cond) {
	if (cond !== undefined && !cond) {
		return
	}

	return __jsapi__.DoGroup(name, fn);
}

/**
 * Runs checks on a value.
 * @param  {any}    val     Value to test.
 * @param  {...Object} sets Sets of tests.
 */
export function check(val, ...sets) {
	return __jsapi__.DoCheck(val, ...sets);
}

/**
 * Sleeps for the specified duration.
 * @param  {Number} secs Duration, in seconds.
 */
export function sleep(secs) {
	__jsapi__.Sleep(secs * 1.0);
}

/**
 * Marks the test as "tainted", meaning it should exit with a nonzero status code. This is done
 * automatically if any check fails, but you can use this to do it manually.
 */
export function taint() {
	__jsapi__.Taint();
}

/**
 * Asserts that a value is truthy.
 * @param  {any}    exp   Expression result.
 * @param  {string} [err] Error message.
 * @throws {Error}        If exp is falsy.
 */
export function _assert(exp, err = "assertion failed") {
	if (!exp) {
		throw new Error(err);
	}
}

export default {
	group: group,
	check: check,
	sleep: sleep,
	taint: taint,
	_assert: _assert,
};
