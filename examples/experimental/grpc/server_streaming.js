import { Client, Stream } from 'k6/experimental/grpc';
import { sleep } from 'k6';

const COORD_FACTOR = 1e7;
// to run this sample, you need to start the grpc server first.
// to start the grpc server, run the following command in k6 repository's root:
// go run -mod=mod examples/grpc_server/*.go
// (golang should be installed)
const GRPC_ADDR = __ENV.GRPC_ADDR || '127.0.0.1:10000';
const GRPC_PROTO_PATH = __ENV.GRPC_PROTO_PATH || '../../grpc_server/route_guide.proto';

let client = new Client();

client.load([], GRPC_PROTO_PATH);

export default () => {
  client.connect(GRPC_ADDR, { plaintext: true });

  const stream = new Stream(client, 'main.FeatureExplorer/ListFeatures', null);

  stream.on('data', function (feature) {
    console.log(
      'Found feature called "' +
        feature.name +
        '" at ' +
        feature.location.latitude / COORD_FACTOR +
        ', ' +
        feature.location.longitude / COORD_FACTOR
    );
  });

  stream.on('end', function () {
    // The server has finished sending
    client.close();
    console.log('All done');
  });

  stream.on('error', function (e) {
    // An error has occurred and the stream has been closed.
    console.log('Error: ' + JSON.stringify(e));
  });

  // send a message to the server
  stream.write({
    lo: {
      latitude: 400000000,
      longitude: -750000000,
    },
    hi: {
      latitude: 420000000,
      longitude: -730000000,
    },
  });

  sleep(0.5);
};
