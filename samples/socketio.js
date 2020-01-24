// import ws from "k6/ws";
import ws from "k6/socketio";
import { check, sleep } from "k6";
// const conversationMessages = ['Hi', 'Hello', 'Another message'];

export default function() {
	// var url = 'ws://demos.kaazing.com/echo';
	// var url = 'ws://localhost:3000/socket.io/?EIO=3&transport=websocket';
	var url =
		"wss://connector.athenka.com/socket.io/?EIO=3&transport=websocket";
	var params = { tags: { my_tag: "hello" } };
	var response = ws.connect(url, params, function(socket) {
		socket.on("open", function open() {
			console.log("connected");
			const msg1 =
				'{"chatId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759-af8b1908-2678-4827-9b69-85bdaf851597","conversationId":"91a0f951-7907-4b0d-b81d-44b1da00407f","senderId":"af8b1908-2678-4827-9b69-85bdaf851597"}';
			const msg2 =
				'{"action":"UNK","dataType":"TEXT","data":"how are you","conversationInfo":{"channelId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759","channelToken":"b9c5d8cc-9a99-439c-9ef2-fb543733947d","recipientId":"b9c5d8cc-9a99-439c-9ef2-fb543733947d","chatId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759-af8b1908-2678-4827-9b69-85bdaf851597","senderId":"af8b1908-2678-4827-9b69-85bdaf851597"},"payload":{}}';
			socket.send("conversationInfo", msg1);
			socket.send("message", msg2);
			//socket.sendSocketIO('message', 'Hello! websocket test' + __VU);
		});

		socket.on("ping", function() {
			console.log("PING!");
		});

		socket.on("pong", function() {
			console.log("PONG!");
		});

		socket.on("handshake", function(data) {
			console.log("message handshake", data);
			// const msg1 =
			// 	'{"chatId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759-af8b1908-2678-4827-9b69-85bdaf851597","conversationId":"91a0f951-7907-4b0d-b81d-44b1da00407f","senderId":"af8b1908-2678-4827-9b69-85bdaf851597"}';
			// const msg2 =
			// 	'{"action":"UNK","dataType":"TEXT","data":"how are you","conversationInfo":{"channelId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759","channelToken":"b9c5d8cc-9a99-439c-9ef2-fb543733947d","recipientId":"b9c5d8cc-9a99-439c-9ef2-fb543733947d","chatId":"259b75a0-6dd4-4b6c-ba92-3d2a0df0d759-af8b1908-2678-4827-9b69-85bdaf851597","senderId":"af8b1908-2678-4827-9b69-85bdaf851597"},"payload":{}}';
			// socket.sendSocketIO("conversationInfo", msg1);
			// socket.sendSocketIO("message", msg2);
			//console.log('message handshake: ', data);
		});

		socket.on("message", function(data) {
			console.log("message response: ", data);
		});

		socket.on("close", function() {
			console.log("disconnected");
		});

		socket.setTimeout(() => {
			console.log("End socket test after 15s");
			socket.close();
		}, 5000);
	});
	// console.log(JSON.stringify(response));
	// response.body = responseMessage;
	// check(response, { 'status is 101': (r) => r && r.status === 101 });
	// check(response, {
	//   'sample check': (r) => {
	//     console.log(JSON.stringify(r));
	//     return true;
	//   }
	// });
}

/**
 *
 */

// import socket from "k6/socketio";

// export default function() {
// 	const response = socket.connect(
// 		"ws://localhost:3000/socket.io/?EIO=3&transport=websocket",
// 		{
// 			headers: {
// 				key1: ["1", "2", "3"],
// 				key2: ["m", "i", "n", "h"],
// 				anotherlongkey: ["test", "domain", "path", "duration"]
// 			},
// 			cookies: {
// 				sample1: "abc",
// 				sample2: "abcxyz",
// 				anotherCookieKey: { value: "hehe", replace: true },
// 				minhhoangcookie: `he abc asd sdsadsad ${Date.now()}`
// 			}
// 		},
// 		function(socket) {
// 			console.log("abc function");
// 			console.log(JSON.stringify(socket));
// 			socket.setTimeout(() => {
// 				console.log("close connection");
// 				socket.close();
// 			}, 3000);
// 		}
// 	);

// 	console.log(JSON.stringify(response));
// }
