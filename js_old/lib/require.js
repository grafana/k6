"use strict";

function require(name) {
	var mod = __internal__.modules[name];
	if (!mod) {
		throw new Error("Unknown module: " + name);
	}
	return mod;
}
