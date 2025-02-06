import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.1.0/index.js';
import { WebSocket } from 'k6/experimental/websockets';

const sessionDuration = randomIntBetween(1000, 3000); // user session between 1s and 3s

export default function () {
	for (let i = 0; i < 4; i++) {
		startWSWorker(i);
	}
}

function startWSWorker(id) {
	// create a new websocket connection
	const ws = new WebSocket(`wss://quickpizza.grafana.com/ws`);
	ws.binaryType = 'arraybuffer';

	ws.addEventListener('open', () => {
		// change the user name
		ws.send(JSON.stringify({ event: 'SET_NAME', new_name: `VU ${__VU}:${id}` }));

		// listen for messages/errors and log them into console
		ws.addEventListener('message', (e) => {
			const msg = JSON.parse(e.data);
			if (msg.event === 'CHAT_MSG') {
				console.log(`VU ${__VU}:${id} received: ${msg.user} says: ${msg.message}`);
			} else if (msg.event === 'ERROR') {
				console.error(`VU ${__VU}:${id} received:: ${msg.message}`);
			} else {
				console.log(`VU ${__VU}:${id} received unhandled message: ${msg.message}`);
			}
		});

		// send a message every 2-8 seconds
		const intervalId = setInterval(() => {
			ws.send(JSON.stringify({ event: 'SAY', message: `I'm saying ${randomString(5)}` }));
		}, randomIntBetween(2000, 8000)); // say something every 2-8 seconds

		// after a sessionDuration stop sending messages and leave the room
		const timeout1id = setTimeout(function () {
			clearInterval(intervalId);
			console.log(`VU ${__VU}:${id}: ${sessionDuration}ms passed, leaving the chat`);
			ws.send(JSON.stringify({ event: 'LEAVE' }));
		}, sessionDuration);

		// after a sessionDuration + 3s close the connection
		const timeout2id = setTimeout(function () {
			console.log(`Closing the socket forcefully 3s after graceful LEAVE`);
			ws.close();
		}, sessionDuration + 3000);

		// when connection is closing, clean up the previously created timers
		ws.addEventListener('close', () => {
			clearTimeout(timeout1id);
			clearTimeout(timeout2id);
			console.log(`VU ${__VU}:${id}: disconnected`);
		});
	});
}
