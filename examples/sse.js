import sse from "k6/experimental/sse";
import {check} from "k6";

export default function () {
	var url = "https://echo.websocket.org/.sse";
	var params = {"tags": {"my_tag": "hello"}};

	var response = sse.open(url, params, function (client) {
		client.on('open', function open() {
			console.log('connected');
		});

		client.on('event', function (event) {
			console.log(`event id=${event.id}, name=${event.name}, data=${event.data}`);
		});

		client.on('close', function close() {
			console.log('disconnected');
		});

		client.on('error', function (e) {
			console.log('An unexpected error occurred: ', e.error());
		});
	});

	check(response, {"status is 200": (r) => r && r.status === 200});
};
