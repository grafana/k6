import grpc from 'k6/net/grpc';
import { check } from "k6";

let client = new grpc.Client();
client.reflect("127.0.0.1:10000", { plaintext: true })


export default () => {
    client.connect("127.0.0.1:10000", { plaintext: true })

    const response = client.invoke("main.RouteGuide/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    })

    check(response, { "status is OK": (r) => r && r.status === grpc.StatusOK });
    console.log(JSON.stringify(response.message))

    client.close()
}

