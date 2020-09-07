import grpc from 'k6/grpc';
import { check } from "k6";

let client = grpc.newClient();
client.load([], "samples/grpc_server/route_guide.proto")


export default () => {
    client.connect("localhost:10000", { plaintext: true })

    const response = client.invokeRPC("main.RouteGuide/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    })

    check(response, { "status is OK": (r) => r && r.status === grpc.StatusOK });

    client.close()
}

