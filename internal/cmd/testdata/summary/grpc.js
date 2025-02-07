import grpc from 'k6/net/grpc';
import {check} from 'k6'

const GRPC_ADDR = __ENV.GRPC_ADDR || '127.0.0.1:10000';
const GRPC_PROTO_PATH = __ENV.GRPC_PROTO_PATH || '../../../lib/testutils/grpcservice/route_guide.proto';

let client = new grpc.Client();

client.load([], GRPC_PROTO_PATH);

export function grpcTest() {
	client.connect(GRPC_ADDR, {plaintext: true});

	const response = client.invoke("main.FeatureExplorer/GetFeature", {
		latitude: 410248224,
		longitude: -747127767
	})

	check(response, {"gRPCC status is OK": (r) => r && r.status === grpc.StatusOK});
	console.log(JSON.stringify(response.message))

	client.close()
}