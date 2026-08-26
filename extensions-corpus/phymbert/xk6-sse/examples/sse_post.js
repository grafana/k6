import sse from "k6/x/sse";
import {check} from "k6";

export default function () {
    const url = "https://echo.websocket.org/.sse";
    const params = {
        method: 'POST',
        body: '{"ping": true}',
        headers: {
            "content-type": "application/json",
            "Authorization": "Bearer XXXX"
        }
    }

    const response = sse.open(url, params, function (client) {
        client.on('event', function (event) {
            console.log(`event id=${event.id}, name=${event.name}, data=${event.data}`);
            if (parseInt(event.id) === 2) {
                client.close()
            }
        })
    })

    check(response, {"status is 200": (r) => r && r.status === 200})
}
