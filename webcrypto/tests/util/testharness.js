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
