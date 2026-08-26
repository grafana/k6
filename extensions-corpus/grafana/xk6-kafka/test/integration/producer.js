// Integration test: produce messages to a broker.
//
// Requires KAFKA_BROKER (run `make broker-up` or `make integration`). The topic
// is created up front (via Connection) so the produce is deterministic on a cold
// broker. A produce failure fails the test.
import { Writer, Connection } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_kafka_producer");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        const writer = new Writer({ brokers: [broker], topic });
        try {
          writer.produce({
            messages: [
              { key: "k1", value: "v1", headers: { source: "xk6" } },
              { value: "no key needed" },
            ],
          });
          verify("produced messages", true);
        } finally {
          writer.close();
        }
      } finally {
        connection.deleteTopic(topic);
      }
    } finally {
      connection.close();
    }
  });
}
