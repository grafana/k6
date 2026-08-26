// Integration test: produce then consume a round-trip.
//
// Requires KAFKA_BROKER (run `make broker-up` or `make integration`). The topic
// is created up front (via Connection) so the round-trip is deterministic on a
// cold broker. A failure to round-trip the messages fails the test.
import { Writer, Reader, Connection, START_OFFSETS_FIRST_OFFSET } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, toStr, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_kafka_roundtrip");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        const writer = new Writer({ brokers: [broker], topic });
        try {
          writer.produce({
            messages: [
              { key: "k1", value: "hello" },
              { key: "k2", value: "world" },
            ],
          });
        } finally {
          writer.close();
        }

        const reader = new Reader({
          brokers: [broker],
          topic,
          partition: 0,
          startOffset: START_OFFSETS_FIRST_OFFSET,
        });
        let got = [];
        try {
          for (let i = 0; i < 5 && got.length < 2; i++) {
            got = got.concat(reader.consume({ limit: 2, expectTimeout: true }));
          }
        } finally {
          reader.close();
        }

        if (!verify("consumed both messages", got.length >= 2)) {
          return;
        }
        const values = got.map((m) => toStr(m.value));
        verify(
          "round-trip values match",
          values.indexOf("hello") !== -1 && values.indexOf("world") !== -1,
        );
      } finally {
        connection.deleteTopic(topic);
      }
    } finally {
      connection.close();
    }
  });
}
