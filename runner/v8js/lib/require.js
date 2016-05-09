__internal__._register = function(mod, obj) {
	if (!(mod in __internal__._modules)) {
		__internal__._modules[mod] = {};
	}
	for (k in Object.keys(obj)) {
		__internal__._modules[mod][k] = obj[k];
	}
}

function require(mod) {
	if (!(mod in __internal__._modules)) {
		throw new Error("module not found: " + mod);
	}
	return __internal__._modules[mod];
}
