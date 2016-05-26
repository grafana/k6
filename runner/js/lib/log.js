"use strict";

__internal__.modules.log.debug = function() {
	return __internal__.modules.log.type.apply(this, _.concat(['debug'], arguments));
}

__internal__.modules.log.info = function() {
	return __internal__.modules.log.type.apply(this, _.concat(['info'], arguments));
}

__internal__.modules.log.warn = function() {
	return __internal__.modules.log.type.apply(this, _.concat(['warn'], arguments));
}

__internal__.modules.log.warning = function() {
	return __internal__.modules.log.warn(arguments);
}

__internal__.modules.log.error = function() {
	return __internal__.modules.log.type.apply(this, _.concat(['error'], arguments));
}
