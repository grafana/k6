"use strict";

function require(name) {
	var mod = __modules__[name];
	if (!mod) {
		throw new Error("Unknown module: " + name);
	}
	return mod;
}
