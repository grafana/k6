import sse from "k6/x/sse"
import {check} from "k6"

export default function () {
    const url = "https://echo.websocket.org/.sse"
    const params = {
        method: 'GET',
        headers: {
            "Authorization": "Bearer XXXX"
        },
        tags: {"my_k6s_tag": "hello sse"}
    }

    const response = sse.open(url, params, function (client) {
        client.on('open', function open() {
            console.log('connected')
        })

        client.on('event', function (event) {
            console.log(`event id=${event.id}, name=${event.name}, data=${event.data}`)
            if (parseInt(event.id) === 4) {
                client.close()
            }
        })

        client.on('error', function (e) {
            console.log('An unexpected error occurred: ', e.error())
        })
    })

    check(response, {"status is 200": (r) => r && r.status === 200})
}
