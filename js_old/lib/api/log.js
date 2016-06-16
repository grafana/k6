"use strict";

$log = $log || {};

$log.debug = function() {
	return $log.log.apply(this, _.concat(['debug'], arguments));
}

$log.info = function() {
	return $log.log.apply(this, _.concat(['info'], arguments));
}

$log.warn = function() {
	return $log.log.apply(this, _.concat(['warn'], arguments));
}

$log.warning = function() {
	return $log.warn.apply(this, arguments);
}

$log.error = function() {
	return $log.log.apply(this, _.concat(['error'], arguments));
}
