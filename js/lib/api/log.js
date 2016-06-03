"use strict";

__modules__.log = __modules__.log || {};

__modules__.log.debug = function() {
	return __modules__.log.log.apply(this, _.concat(['debug'], arguments));
}

__modules__.log.info = function() {
	return __modules__.log.log.apply(this, _.concat(['info'], arguments));
}

__modules__.log.warn = function() {
	return __modules__.log.log.apply(this, _.concat(['warn'], arguments));
}

__modules__.log.warning = function() {
	return __modules__.log.warn.apply(this, arguments);
}

__modules__.log.error = function() {
	return __modules__.log.log.apply(this, _.concat(['error'], arguments));
}
