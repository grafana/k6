import grpc from 'k6/net/grpc';
import { check } from 'k6';

// to run this sample, you need to start the grpc server first.
// to start the grpc server, run the following command in k6 repository's root:
// go run -mod=mod examples/grpc_server/*.go
// (golang should be installed)
const GRPC_ADDR = __ENV.GRPC_ADDR || '127.0.0.1:10000';
const GRPC_PROTO_PATH =
  __ENV.GRPC_PROTO_PATH || '../internal/lib/testutils/grpcservice/route_guide.proto';

const client = new grpc.Client();
client.load([], GRPC_PROTO_PATH);

export default () => {
  client.connect(GRPC_ADDR, { plaintext: true });

  // Sending latitude=0, longitude=0 triggers the server to return an
  // InvalidArgument error with structured details (ErrorInfo + BadRequest).
  const response = client.invoke('main.FeatureExplorer/GetFeature', {
    latitude: 0,
    longitude: 0,
  });

  check(response, {
    'status is InvalidArgument': (r) => r && r.status === grpc.StatusInvalidArgument,
    'error message is set': (r) => r.error && r.error.message === 'invalid coordinates',
    'error details are present': (r) => r.error.details && r.error.details.length === 2,
    'first detail is ErrorInfo': (r) =>
      r.error.details[0]['@type'].includes('ErrorInfo') &&
      r.error.details[0].reason === 'ZERO_COORDINATES' &&
      r.error.details[0].domain === 'k6.io/grpc',
    'second detail is BadRequest': (r) =>
      r.error.details[1]['@type'].includes('BadRequest') &&
      r.error.details[1].fieldViolations.length === 2,
  });

  client.close();
};
