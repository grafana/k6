__internal__ = {
	_modules: {},
	_data: {},
};

__internal__._invoke = function(mod, fn, args, async) {
	var send = async ? $send : $sendSync
	var res = send(JSON.stringify({ m: mod, f: fn, a: args }));
	if (res) {
		var obj = JSON.parse(res);
		if (obj._error) {
			throw new Error(obj._error);
		}
		return obj;
	}
}

$recvSync(function(raw) {
	if (raw == 'run') {
		__run__();
	}
});
