import grpc from 'k6/grpc';

export let options = {
  vus: 100,
  duration: '10s',
};

grpc.load([], "samples/grpc_server/route_guide.proto")

export default function() {
    let client = grpc.newClient();

    if(client.connect("localhost:10000")) {
        console.error(err)
        return
    }
    const resp = client.invokeRPC("main.RouteGuide/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    })
    // console.log(resp)
    client.invokeRPC("main.RouteGuide/GetFeature", {
        latitude: 0,
        longitude: 0
    })


    client.close()
}
