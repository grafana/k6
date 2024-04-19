// This file contains a partial adaptation of the testharness.js implementation from
// the W3C WebCrypto API test suite. It is not intended to be a complete
// implementation, but rather a minimal set of functions to support the
// tests for this extension.
//
// Some of the function have been modified to support the k6 javascript runtime,
// and to limit its dependency to the rest of the W3C WebCrypto API test suite internal
// codebase.
//
// The original testharness.js implementation is available at:
// https://github.com/web-platform-tests/wpt/blob/3a3453c62176c97ab51cd492553c2dacd24366b1/resources/testharness.js

/**
 * Utility functions.
 */
function test(func, name) {
	try {
		func();
	} catch (e) {
		throw `${name} failed - ${e}`;
	}
}

function promise_test(func, name) {
	func().catch((e) => {
		throw `${name} failed - ${e}`;
	});
}

/**
 * Make a copy of a Promise in the current realm.
 *
 * @param {Promise} promise the given promise that may be from a different
 *                          realm
 * @returns {Promise}
 *
 * An arbitrary promise provided by the caller may have originated
 * in another frame that have since navigated away, rendering the
 * frame's document inactive. Such a promise cannot be used with
 * `await` or Promise.resolve(), as microtasks associated with it
 * may be prevented from being run. See `issue
 * 5319<https://github.com/whatwg/html/issues/5319>`_ for a
 * particular case.
 *
 * In functions we define here, there is an expectation from the caller
 * that the promise is from the current realm, that can always be used with
 * `await`, etc. We therefore create a new promise in this realm that
 * inherit the value and status from the given promise.
 */

function bring_promise_to_current_realm(promise) {
	return new Promise(promise.then.bind(promise));
}

/**
 * @class
 * Exception type that represents a failing assert.
 *
 * @param {string} message - Error message.
 */
function AssertionError(message) {
	if (typeof message == "string") {
		message = sanitize_unpaired_surrogates(message);
	}
	this.message = message;
	this.stack = get_stack();
}

AssertionError.prototype = Object.create(Error.prototype);

function assert(expected_true, function_name, description, error, substitutions) {
	if (expected_true !== true) {
		var msg = make_message(function_name, description,
			error, substitutions);
		throw new AssertionError(msg);
	}
}


// NOTE: This is a simplified version of the original implementation
// found at: https://github.com/web-platform-tests/wpt/blob/e955fbc72b5a98e1c2dc6a6c1a048886c8a99785/resources/testharness.js#L4615
function make_message(function_name, description, error, substitutions) {
	// for (var p in substitutions) {
	// 	if (substitutions.hasOwnProperty(p)) {
	// 		substitutions[p] = format_value(substitutions[p]);
	// 	}
	// }
	var node_form = substitute(["{text}", "${function_name}: ${description}" + error],
		merge({
				function_name: function_name,
				description: (description ? description + " " : "")
			},
			substitutions));
	return node_form.slice(1).join("");
}

function is_single_node(template) {
	return typeof template[0] === "string";
}

function substitute(template, substitutions) {
	if (typeof template === "function") {
		var replacement = template(substitutions);
		if (!replacement) {
			return null;
		}

		return substitute(replacement, substitutions);
	}

	if (is_single_node(template)) {
		return substitute_single(template, substitutions);
	}

	return filter(map(template, function (x) {
		return substitute(x, substitutions);
	}), function (x) {
		return x !== null;
	});
}

function substitute_single(template, substitutions) {
	var substitution_re = /\$\{([^ }]*)\}/g;

	function do_substitution(input) {
		var components = input.split(substitution_re);
		var rv = [];
		for (var i = 0; i < components.length; i += 2) {
			rv.push(components[i]);
			if (components[i + 1]) {
				rv.push(String(substitutions[components[i + 1]]));
			}
		}
		return rv;
	}

	function substitute_attrs(attrs, rv) {
		rv[1] = {};
		for (var name in template[1]) {
			if (attrs.hasOwnProperty(name)) {
				var new_name = do_substitution(name).join("");
				var new_value = do_substitution(attrs[name]).join("");
				rv[1][new_name] = new_value;
			}
		}
	}

	function substitute_children(children, rv) {
		for (var i = 0; i < children.length; i++) {
			if (children[i] instanceof Object) {
				var replacement = substitute(children[i], substitutions);
				if (replacement !== null) {
					if (is_single_node(replacement)) {
						rv.push(replacement);
					} else {
						extend(rv, replacement);
					}
				}
			} else {
				extend(rv, do_substitution(String(children[i])));
			}
		}
		return rv;
	}

	var rv = [];
	rv.push(do_substitution(String(template[0])).join(""));

	if (template[0] === "{text}") {
		substitute_children(template.slice(1), rv);
	} else {
		substitute_attrs(template[1], rv);
		substitute_children(template.slice(2), rv);
	}

	return rv;
}

function filter(array, callable, thisObj) {
	var rv = [];
	for (var i = 0; i < array.length; i++) {
		if (array.hasOwnProperty(i)) {
			var pass = callable.call(thisObj, array[i], i, array);
			if (pass) {
				rv.push(array[i]);
			}
		}
	}
	return rv;
}

function map(array, callable, thisObj) {
	var rv = [];
	rv.length = array.length;
	for (var i = 0; i < array.length; i++) {
		if (array.hasOwnProperty(i)) {
			rv[i] = callable.call(thisObj, array[i], i, array);
		}
	}
	return rv;
}

function extend(array, items) {
	Array.prototype.push.apply(array, items);
}

function forEach(array, callback, thisObj) {
	for (var i = 0; i < array.length; i++) {
		if (array.hasOwnProperty(i)) {
			callback.call(thisObj, array[i], i, array);
		}
	}
}

function merge(a, b) {
	var rv = {};
	var p;
	for (p in a) {
		rv[p] = a[p];
	}
	for (p in b) {
		rv[p] = b[p];
	}
	return rv;
}

/**
 * Assert that ``actual`` is the same value as ``expected``.
 *
 * For objects this compares by cobject identity; for primitives
 * this distinguishes between 0 and -0, and has correct handling
 * of NaN.
 *
 * @param {Any} actual - Test value.
 * @param {Any} expected - Expected value.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_equals(actual, expected, description) {
	if (actual !== expected) {
		throw `assert_equals ${description} expected (${typeof expected}) ${expected} but got (${typeof actual}) ${actual}`;
	}
}

/**
 * Assert that ``actual`` is not the same value as ``expected``.
 *
 * Comparison is as for :js:func:`assert_equals`.
 *
 * @param {Any} actual - Test value.
 * @param {Any} expected - The value ``actual`` is expected to be different to.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_not_equals(actual, expected, description) {
	if (actual === expected) {
		throw `assert_not_equals ${description} got disallowed value ${actual}`;
	}
}

/**
 * Assert that ``actual`` is strictly true
 *
 * @param {Any} actual - Value that is asserted to be true
 * @param {string} [description] - Description of the condition being tested
 */
function assert_true(actual, description) {
	if (!actual) {
		throw `assert_true ${description} expected true got ${actual}`;
	}
}

/**
 * Assert that ``actual`` is strictly false
 *
 * @param {Any} actual - Value that is asserted to be false
 * @param {string} [description] - Description of the condition being tested
 */
function assert_false(actual, description) {
	if (actual) {
		throw `assert_true ${description} expected false got ${actual}`;
	}
}

/**
 * Assert that ``expected`` is an array and ``actual`` is one of the members.
 * This is implemented using ``indexOf``, so doesn't handle NaN or ±0 correctly.
 *
 * @param {Any} actual - Test value.
 * @param {Array} expected - An array that ``actual`` is expected to
 * be a member of.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_in_array(actual, expected, description) {
	if (expected.indexOf(actual) === -1) {
		throw `assert_in_array ${description} value ${actual} not in array ${expected}`;
	}
}

/**
 * Asserts if called. Used to ensure that a specific codepath is
 * not taken e.g. that an error event isn't fired.
 *
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_unreached(description) {
	throw `reached unreachable code, reason: ${description}`
}

/**
 * Assert that ``actual`` and ``expected`` are both arrays, and that the array properties of
 * ``actual`` and ``expected`` are all the same value (as for :js:func:`assert_equals`).
 *
 * @param {Array} actual - Test array.
 * @param {Array} expected - Array that is expected to contain the same values as ``actual``.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_array_equals(actual, expected, description) {
	if (typeof actual !== "object" || actual === null || !("length" in actual)) {
		throw `assert_array_equals ${description} value is ${actual}, expected array`;
	}

	if (actual.length !== expected.length) {
		throw `assert_array_equals ${description} lengths differ, expected array ${expected} length ${expected.length}, got ${actual} length ${actual.length}`;
	}

	for (var i = 0; i < actual.length; i++) {
		if (actual.hasOwnProperty(i) !== expected.hasOwnProperty(i)) {
			throw `assert_array_equals ${description} expected property ${i} to be ${expected.hasOwnProperty(i)} but was ${actual.hasOwnProperty(i)} (expected array ${expected} got ${actual})`;
		}

		if (!same_value(expected[i], actual[i])) {
			throw `assert_array_equals ${description} expected property ${i} to be ${expected[i]} but got ${actual[i]} (expected array ${expected} got ${actual})`;
		}
	}
}

/**
 * Assert that ``actual`` is a number greater than ``expected``.
 *
 * @param {number} actual - Test value.
 * @param {number} expected - Number that ``actual`` must be greater than.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_greater_than(actual, expected, description)
{
	/*
	 * Test if a primitive number is greater than another
	 */
	assert(typeof actual === "number",
		"assert_greater_than", description,
		"expected a number but got a ${type_actual}",
		{type_actual:typeof actual});

	assert(actual > expected,
		"assert_greater_than", description,
		"expected a number greater than ${expected} but got ${actual}",
		{expected:expected, actual:actual});
}

/**
 * Assert the provided value is thrown.
 *
 * @param {value} exception The expected exception.
 * @param {Function} func Function which should throw.
 * @param {string} [description] Error description for the case that the error is not thrown.
 */
function assert_throws_exactly(exception, func, description) {
	assert_throws_exactly_impl(exception, func, description,
		"assert_throws_exactly");
}

/**
 * Like assert_throws_exactly but allows specifying the assertion type
 * (assert_throws_exactly or promise_rejects_exactly, in practice).
 */
function assert_throws_exactly_impl(exception, func, description,
                                    assertion_type) {
	try {
		func.call(this);
		assert(false, assertion_type, description,
			"${func} did not throw", {func: func});
	} catch (e) {
		if (e instanceof AssertionError) {
			throw e;
		}

		assert(same_value(e, exception), assertion_type, description,
			"${func} threw ${e} but we expected it to throw ${exception}",
			{func: func, e: e, exception: exception});
	}
}

/**
 * Assert that a Promise is rejected with the provided value.
 *
 * @param {Test} test - the `Test` to use for the assertion.
 * @param {Any} exception - The expected value of the rejected promise.
 * @param {Promise} promise - The promise that's expected to
 * reject.
 * @param {string} [description] Error message to add to assert in case of
 *                               failure.
 */
function promise_rejects_exactly(test, exception, promise, description) {
	return promise.then(
		() => {
			throw new Error("Should have rejected: " + description);
		},
		(e) => {
			assert_throws_exactly_impl(exception, function () {
					throw e
				},
				description, "promise_rejects_exactly");
		}
	);
}

/**
 * Assert that a Promise is rejected with the right ECMAScript exception.
 *
 * @param {Test} test - the `Test` to use for the assertion.
 * @param {Function} constructor - The expected exception constructor.
 * @param {Promise} promise - The promise that's expected to
 * reject with the given exception.
 * @param {string} [description] Error message to add to assert in case of
 *                               failure.
 */
function promise_rejects_js(test, constructor, promise, description) {
	return bring_promise_to_current_realm(promise)
		.then(() => {assert_unreached("Should have rejected: " + description)})
		.catch(function(e) {
			assert_throws_js_impl(constructor, function() { throw e },
				description, "promise_rejects_js");
		});
}

/**
 * Assert a JS Error with the expected constructor is thrown.
 *
 * @param {object} constructor The expected exception constructor.
 * @param {Function} func Function which should throw.
 * @param {string} [description] Error description for the case that the error is not thrown.
 */
function assert_throws_js(constructor, func, description) {
	assert_throws_js_impl(constructor, func, description,
		"assert_throws_js");
}

/**
 * Like assert_throws_js but allows specifying the assertion type
 * (assert_throws_js or promise_rejects_js, in practice).
 */
function assert_throws_js_impl(constructor, func, description,
                               assertion_type) {
	try {
		func.call(this);
		assert(false, assertion_type, description,
			"${func} did not throw", {func: func});
	} catch (e) {
		if (e instanceof AssertionError) {
			throw e;
		}

		// Basic sanity-checks on the thrown exception.
		assert(typeof e === "object",
			assertion_type, description,
			"${func} threw ${e} with type ${type}, not an object",
			{func: func, e: e, type: typeof e});

		assert(e !== null,
			assertion_type, description,
			"${func} threw null, not an object",
			{func: func});

		// Note @oleiade: As k6 does not throw error objects that match the Javascript
		// standard errors and their associated expectations and properties, we cannot
		// rely on the WPT assertions to be true.
		//
		// Instead, we check that the error object has the shape we give it when we throw it.
		// Namely, that it has a name property that matches the name of the expected constructor.

		assert('name' in e,
			assertion_type, description,
			"${func} threw ${e} without a name property",
			{func: func, e: e});

		assert(e.name === constructor.name,
			assertion_type, description,
			"${func} threw ${e} with name ${e.name}, not ${constructor.name}",
			{func: func, e: e, constructor: constructor});

		// Note @oleiade: We deactivated the following assertions in favor of our own
		// as mentioned above.

		// Basic sanity-check on the passed-in constructor
		// assert(typeof constructor == "function",
		// 	assertion_type, description,
		// 	"${constructor} is not a constructor",
		// 	{constructor:constructor});
		// var obj = constructor;
		// while (obj) {
		// 	if (typeof obj === "function" &&
		// 		obj.name === "Error") {
		// 		break;
		// 	}
		// 	obj = Object.getPrototypeOf(obj);
		// }
		// assert(obj != null,
		// 	assertion_type, description,
		// 	"${constructor} is not an Error subtype",
		// 	{constructor:constructor});
		//
		// // And checking that our exception is reasonable
		// assert(e.constructor === constructor &&
		// 	e.name === constructor.name,
		// 	assertion_type, description,
		// 	"${func} threw ${actual} (${actual_name}) expected instance of ${expected} (${expected_name})",
		// 	{func:func, actual:e, actual_name:e.name,
		// 		expected:constructor,
		// 		expected_name:constructor.name});
	}
}

function same_value(x, y) {
	if (y !== y) {
		//NaN case
		return x !== x;
	}
	if (x === 0 && y === 0) {
		//Distinguish +0 and -0
		return 1 / x === 1 / y;
	}
	// Note @joanlopez: We cannot rely on the WPT assertions implementation, because
	// k6 does not throw error objects that match the JavaScript standard errors and
	// their associated expectations and properties.
	if (isObject(x) && isObject(y)) {
		return areObjectsEquivalent(x, y);
	}
	return x === y;
}

function isObject(object) {
	return typeof object === 'object' && object !== null;
}

function areObjectsEquivalent(obj1, obj2) {
	const obj1Keys = Object.keys(obj1);
	const obj2Keys = Object.keys(obj2);

	if (obj1Keys.length !== obj2Keys.length) {
		return false; // They have different numbers of keys
	}

	for (let key of obj1Keys) {
		const val1 = obj1[key];
		const val2 = obj2[key];
		const areObjects = isObject(val1) && isObject(val2);
		if (
			(areObjects && !areObjectsEquivalent(val1, val2)) ||
			(!areObjects && val1 !== val2)
		) {
			return false; // Either the values are not equal or, if objects, not equivalent
		}
	}
	return true; // Everything matched
}

// This function was deprecated in July of 2015.
// See https://github.com/web-platform-tests/wpt/issues/2033
/**
 * @deprecated
 * Recursively compare two objects for equality.
 *
 * See `Issue 2033
 * <https://github.com/web-platform-tests/wpt/issues/2033>`_ for
 * more information.
 *
 * @param {Object} actual - Test value.
 * @param {Object} expected - Expected value.
 * @param {string} [description] - Description of the condition being tested.
 */
function assert_object_equals(actual, expected, description)
{
	assert(typeof actual === "object" && actual !== null, "assert_object_equals", description,
		"value is ${actual}, expected object",
		{actual: actual});
	//This needs to be improved a great deal
	function check_equal(actual, expected, stack)
	{
		stack.push(actual);

		var p;
		for (p in actual) {
			assert(expected.hasOwnProperty(p), "assert_object_equals", description,
				"unexpected property ${p}", {p:p});

			if (typeof actual[p] === "object" && actual[p] !== null) {
				if (stack.indexOf(actual[p]) === -1) {
					check_equal(actual[p], expected[p], stack);
				}
			} else {
				assert(same_value(actual[p], expected[p]), "assert_object_equals", description,
					"property ${p} expected ${expected} got ${actual}",
					{p:p, expected:expected[p], actual:actual[p]});
			}
		}
		for (p in expected) {
			assert(actual.hasOwnProperty(p),
				"assert_object_equals", description,
				"expected property ${p} missing", {p:p});
		}
		stack.pop();
	}
	check_equal(actual, expected, []);
}

function code_unit_str(char) {
	return 'U+' + char.charCodeAt(0).toString(16);
}

function sanitize_unpaired_surrogates(str) {
	return str.replace(
		/([\ud800-\udbff]+)(?![\udc00-\udfff])|(^|[^\ud800-\udbff])([\udc00-\udfff]+)/g,
		function (_, low, prefix, high) {
			var output = prefix || "";  // prefix may be undefined
			var string = low || high;  // only one of these alternates can match
			for (var i = 0; i < string.length; i++) {
				output += code_unit_str(string[i]);
			}
			return output;
		});
}

const get_stack = function () {
	var stack = new Error().stack;

	// 'Error.stack' is not supported in all browsers/versions
	if (!stack) {
		return "(Stack trace unavailable)";
	}

	var lines = stack.split("\n");

	// Create a pattern to match stack frames originating within testharness.js.  These include the
	// script URL, followed by the line/col (e.g., '/resources/testharness.js:120:21').
	// Escape the URL per http://stackoverflow.com/questions/3561493/is-there-a-regexp-escape-function-in-javascript
	// in case it contains RegExp characters.
	// NOTE @oleiade: We explicitly bypass the get_script_url operation as it's specific to the
	// web platform test suite and enforce the use of an empty string instead.
	// var script_url = get_script_url();
	var script_url = '';
	var re_text = script_url ? script_url.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&') : "\\btestharness.js";
	var re = new RegExp(re_text + ":\\d+:\\d+");

	// Some browsers include a preamble that specifies the type of the error object.  Skip this by
	// advancing until we find the first stack frame originating from testharness.js.
	var i = 0;
	while (!re.test(lines[i]) && i < lines.length) {
		i++;
	}

	// Then skip the top frames originating from testharness.js to begin the stack at the test code.
	while (re.test(lines[i]) && i < lines.length) {
		i++;
	}

	// Paranoid check that we didn't skip all frames.  If so, return the original stack unmodified.
	if (i >= lines.length) {
		return stack;
	}

	return lines.slice(i).join("\n");
}

// Internal helper function to provide timeout-like functionality in
// environments where there is no setTimeout(). (No timeout ID or
// clearTimeout().)
function fake_set_timeout(callback, delay) {
	var p = Promise.resolve();
	var start = Date.now();
	var end = start + delay;

	function check() {
		if ((end - Date.now()) > 0) {
			p.then(check);
		} else {
			callback();
		}
	}

	p.then(check);
}


/**
 * Global version of :js:func:`Test.step_timeout` for use in single page tests.
 *
 * @param {Function} func - Function to run after the timeout
 * @param {number} timeout - Time in ms to wait before running the
 * test step. The actual wait time is ``timeout`` x
 * ``timeout_multiplier`` (NOTE: Set to 1 for simplicity).
 */
function step_timeout(func, timeout) {
	var outer_this = this;
	var args = Array.prototype.slice.call(arguments, 2);
	var local_set_timeout = typeof this.setTimeout === "undefined" ? fake_set_timeout : setTimeout;
	return local_set_timeout(function () {
		func.apply(outer_this, args);
	}, timeout);
}