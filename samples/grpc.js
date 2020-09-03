import grpc from 'k6/grpc';


export function setup() {
    const err = grpc.load([], "samples/grpc_server/route_guide.proto")
    if (err) {
        console.error(err)
    }

    if(grpc.connect("localhost:10000")) {
        console.error(err)
    }
}

export default function() {
    // Do something here
    console.log(grpc.invokeRPC("main.RouteGuide/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    }))
}
