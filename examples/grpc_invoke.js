import grpc from 'k6/net/grpc';
import { check } from "k6";

// to run this sample, you need to start the grpc server first.
// to start the grpc server, run the following command in k6 repository's root:
// go run -mod=mod examples/grpc_server/*.go
// (golang should be installed)
const GRPC_ADDR = __ENV.GRPC_ADDR || '127.0.0.1:10000';
const GRPC_PROTO_PATH = __ENV.GRPC_PROTO_PATH || '../internal/lib/testutils/grpcservice/route_guide.proto';

let client = new grpc.Client();

client.load([], GRPC_PROTO_PATH);

export default () => {
  client.connect(GRPC_ADDR, { plaintext: true });

    const response = client.invoke("main.FeatureExplorer/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    })

    check(response, { "status is OK": (r) => r && r.status === grpc.StatusOK });
    console.log(JSON.stringify(response.message))

    client.close()
}

