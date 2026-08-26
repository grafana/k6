// Integration test: connect to a real broker and close.
//
// Requires KAFKA_BROKER (run `make broker-up` or `make integration`). An
// unreachable broker makes `new Connection` throw and fails the test.
import { Connection } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const connection = new Connection({ address: broker });
    verify("connected to broker", true);
    connection.close();
  });
}
