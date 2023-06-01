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

// a sample DB of points
const DB = [
  {
    location: { latitude: 407838351, longitude: -746143763 },
    name: 'Patriots Path, Mendham, NJ 07945, USA',
  },
  {
    location: { latitude: 408122808, longitude: -743999179 },
    name: '101 New Jersey 10, Whippany, NJ 07981, USA',
  },
  {
    location: { latitude: 413628156, longitude: -749015468 },
    name: 'U.S. 6, Shohola, PA 18458, USA',
  },
  {
    location: { latitude: 419999544, longitude: -740371136 },
    name: '5 Conners Road, Kingston, NY 12401, USA',
  },
  {
    location: { latitude: 414008389, longitude: -743951297 },
    name: 'Mid Hudson Psychiatric Center, New Hampton, NY 10958, USA',
  },
  {
    location: { latitude: 419611318, longitude: -746524769 },
    name: '287 Flugertown Road, Livingston Manor, NY 12758, USA',
  },
  {
    location: { latitude: 406109563, longitude: -742186778 },
    name: '4001 Tremley Point Road, Linden, NJ 07036, USA',
  },
  {
    location: { latitude: 416802456, longitude: -742370183 },
    name: '352 South Mountain Road, Wallkill, NY 12589, USA',
  },
  {
    location: { latitude: 412950425, longitude: -741077389 },
    name: 'Bailey Turn Road, Harriman, NY 10926, USA',
  },
  {
    location: { latitude: 412144655, longitude: -743949739 },
    name: '193-199 Wawayanda Road, Hewitt, NJ 07421, USA',
  },
];

// to run this sample, you need to start the grpc server first.
// to start the grpc server, run the following command in k6 repository's root:
// go run -mod=mod examples/grpc_server/*.go
// (golang should be installed)

// the example below is based on the original GRPC client streaming example
//
// It sends several randomly chosen points from the pre-generated
// feature database with a variable delay in between. Prints the
// statistics when they are sent from the server.
export default () => {
  if (__ITER == 0) {
    client.connect(GRPC_ADDR, { plaintext: true });
  }

  const stream = new Stream(client, 'main.RouteGuide/RecordRoute');

  stream.on('data', (stats) => {
    console.log('Finished trip with', stats.pointCount, 'points');
    console.log('Passed', stats.featureCount, 'features');
    console.log('Travelled', stats.distance, 'meters');
    console.log('It took', stats.elapsedTime, 'seconds');
  });

  stream.on('error', (err) => {
    console.log('Stream Error: ' + JSON.stringify(err));
  });

  stream.on('end', () => {
    client.close();
    console.log('All done');
  })

  // send 5 random items
  for (var i = 0; i < 5; i++) {
    let point = DB[Math.floor(Math.random() * DB.length)];
    pointSender(stream, point);
  }

  // close the client stream
  stream.end();

  sleep(1);
};

const pointSender = (stream, point) => {
  console.log(
    'Visiting point ' +
      point.name +
      ' ' +
      point.location.latitude / COORD_FACTOR +
      ', ' +
      point.location.longitude / COORD_FACTOR
  );

  // send the location to the server
  stream.write(point.location);

  sleep(0.5);
};
