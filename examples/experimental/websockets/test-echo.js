import { WebSocket, EventName, ReadyState, BinaryType } from "k6/experimental/websockets"

export default function() {
	// local echo server should be launched with `make ws-echo-server-run`
	var url = "wss://echo.websocket.org/"
	var params = { "tags": { "my_tag": "hello" } };

	let ws = new WebSocket(url, null, params)

	ws.addEventListener(EventName.Open, () => {
		console.log('Connected');
	})

	ws.binaryType = BinaryType.ArrayBuffer;
	ws.onopen = () => {
		console.log('connected')
		ws.send(Date.now().toString())
	}

	let intervalId = setInterval(() => {
		ws.ping();
		console.log("Pinging every 1 sec (setInterval test)")
	}, 1000);

	let timeout1id = setTimeout(function() {
		console.log('3 seconds passed, closing the socket')
		clearInterval(intervalId)
		ws.close()

	}, 3000);

	ws.onclose = () => {
		clearTimeout(timeout1id);

		console.log('disconnected')
	}


	ws.onping = () => {
		console.log("PING!")
	}

	ws.onpong = () => {
		console.log("PONG!")
	}

	// Multiple event handlers on the same event
	ws.addEventListener(EventName.Pong, () => {
		console.log("OTHER PONG!")
	})

	ws.onmessage = (m) => {
		let parsed = parseInt(m.data, 10)
		if (Number.isNaN(parsed)) {
			console.log('Not a number received: ', m.data)

			return
		}

		console.log(`Roundtrip time: ${Date.now() - parsed} ms`);

		let timeoutId = setTimeout(function() {
			if (ws.readyState == ReadyState.Closed) {
				console.log("Socket closed, not sending anything");

				clearTimeout(timeoutId);
				return;
			}

			ws.send(Date.now().toString())
		}, 500);
	}

	ws.onerror = (e) => {
		if (e.error != "websocket: close sent") {
			console.log('An unexpected error occurred: ', e.error);
		}
	};
};
