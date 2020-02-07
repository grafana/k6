import ws from "k6/socketio";

export default function() {
	const url =
		"wss://socket-io-chat.now.sh/socket.io/?EIO=3&transport=websocket";
	const params = { tags: { my_tag: "hello" } };
	const response = ws.connect(url, params, function(socket) {
		let counter = 0;
		socket.on("open", function open() {
			console.log("connected");
			socket.send("add user", `minhhoang_${__VU}`);
		});
		socket.setInterval(function() {
			counter += 1;
			socket.send(
				"new message",
				`k6.io test from minhhoang_${__VU} message ${counter} at ${Date.now()}`
			);
			console.log("count number ", counter);
			if (counter > 300) {
				socket.close();
			}
		}, 5000);

		socket.on("ping", function() {
			console.log("PING!");
		});

		socket.on("pong", function() {
			console.log("PONG!");
		});

		socket.on("handshake", function(data) {
			console.log("message in handshake: ", data);
		});

		socket.on("message", function(data) {
			console.log("message response in [message] event: ", data);
		});

		socket.on("typing", function(data) {
			console.log("message response in [typing] event: ", data);
		});

		socket.on("login", function(data) {
			console.log("message response in [login] event: ", data);
		});

		socket.on("stop typing", function(data) {
			console.log("message response in [stop typing] event: ", data);
		});

		socket.on("new message", function(data) {
			console.log("message response in [new message] event: ", data);
		});

		socket.on("user left", function(data) {
			console.log("message response in [user left] event: ", data);
		});

		socket.on("user joined", function(data) {
			console.log("message response in [joined] event: ", data);
		});

		socket.on("close", function(data) {
			console.log("disconnected");
		});

		socket.setTimeout(() => {
			console.log("End socket test after 15s");
			socket.close();
		}, 30000);

		console.log(response);
	});
}
