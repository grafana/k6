speedboat = {
	_modules: {},
	_require: {},
};

speedboat._require.float64 = function(v) {
	out = parseFloat(v);
	if (isNaN(out)) {
		throw new Error("not a float: " + v);
	}
	return out
}
speedboat._require.float32 = speedboat._require.float64

speedboat._require.int = function(v) {
	out = parseInt(v);
	if (isNaN(out)) {
		throw new Error("not an int: " + v);
	}
	return out
}

speedboat._require.string = function(v) {
	return (v || "").toString();
}

$recvSync(function(raw) {
	if (raw == 'run') {
		__run__();
	}
});
