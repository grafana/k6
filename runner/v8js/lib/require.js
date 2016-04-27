function require(mod) {
	if (!(mod in speedboat._modules)) {
		throw new Error("module not found: " + mod);
	}
	return speedboat._modules[mod];
}
