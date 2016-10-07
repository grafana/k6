console = {
	log(msg, ...args) { console.info(msg, ...args); },

	debug(msg, ...args) { __jsapi__.Log(0, msg, args); },
	info(msg, ...args) { __jsapi__.Log(1, msg, args); },
	warn(msg, ...args) { __jsapi__.Log(2, msg, args); },
	error(msg, ...args) { __jsapi__.Log(3, msg, args); },
};
