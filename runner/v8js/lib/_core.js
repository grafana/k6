speedboat = {
	_internal: {
		recv: {},
	},
};

$recvSync(function(msg) {
	d = JSON.parse(msg);
	fn = speedboat._internal.recv[d.call];
	if (fn !== undefined) {
		fn.apply(speedboat, d.args);
	}
});
