import {
	randomString,
	randomIntBetween,
} from "https://jslib.k6.io/k6-utils/1.1.0/index.js";
import { WebSocket } from "k6/experimental/websockets";
import {
	setTimeout,
	clearTimeout,
	setInterval,
	clearInterval,
} from "k6/experimental/timers";

let chatRoomName = "publicRoom"; // choose your chat room name
let sessionDuration = randomIntBetween(5000, 60000); // user session between 5s and 1m

export default function () {
	for (let i = 0; i < 4; i++) {
		startWSWorker(i);
	}
}

function startWSWorker(id) {
	let url = `wss://test-api.k6.io/ws/crocochat/${chatRoomName}/`;
	let ws = new WebSocket(url);
	ws.addEventListener("open", () => {
		ws.send(
			JSON.stringify({
				event: "SET_NAME",
				new_name: `Croc ${__VU}:${id}`,
			})
		);

		ws.addEventListener("message", (e) => {
			let msg = JSON.parse(e.data);
			if (msg.event === "CHAT_MSG") {
				console.log(
					`VU ${__VU}:${id} received: ${msg.user} says: ${msg.message}`
				);
			} else if (msg.event === "ERROR") {
				console.error(`VU ${__VU}:${id} received:: ${msg.message}`);
			} else {
				console.log(
					`VU ${__VU}:${id} received unhandled message: ${msg.message}`
				);
			}
		});

		let intervalId = setInterval(() => {
			ws.send(
				JSON.stringify({
					event: "SAY",
					message: `I'm saying ${randomString(5)}`,
				})
			);
		}, randomIntBetween(2000, 8000)); // say something every 2-8seconds

		let timeout1id = setTimeout(function () {
			clearInterval(intervalId);
			console.log(
				`VU ${__VU}:${id}: ${sessionDuration}ms passed, leaving the chat`
			);
			ws.send(JSON.stringify({ event: "LEAVE" }));
		}, sessionDuration);

		let timeout2id = setTimeout(function () {
			console.log(
				`Closing the socket forcefully 3s after graceful LEAVE`
			);
			ws.close();
		}, sessionDuration + 3000);

		ws.addEventListener("close", () => {
			clearTimeout(timeout1id);
			clearTimeout(timeout2id);
			console.log(`VU ${__VU}:${id}: disconnected`);
		});
	});
}
