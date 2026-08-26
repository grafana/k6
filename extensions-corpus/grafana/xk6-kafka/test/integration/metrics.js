// Integration test: custom k6 metrics are emitted for produce and consume.
//
// Requires KAFKA_BROKER (run `make broker-up` or `make integration`). The
// thresholds fail the run if the writer/reader message-count metrics are not
// emitted, proving the metrics reach the summary end-to-end.
import { Writer, Reader, Connection, START_OFFSETS_FIRST_OFFSET } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = {
  thresholds: {
    ...thresholds,
    // These fail if the metrics are absent from the summary.
    kafka_writer_message_count: ["count>0"],
    kafka_reader_message_count: ["count>0"],
    kafka_writer_error_count: ["count==0"],
  },
};

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_metrics");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        const writer = new Writer({ brokers: [broker], topic });
        try {
          writer.produce({
            messages: [
              { key: "k1", value: "v1" },
              { key: "k2", value: "v2" },
              { key: "k3", value: "v3" },
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
        try {
          let got = [];
          for (let i = 0; i < 5 && got.length < 3; i++) {
            got = got.concat(reader.consume({ limit: 3, expectTimeout: true }));
          }
          verify("consumed produced messages", got.length >= 3);
        } finally {
          reader.close();
        }
      } finally {
        connection.deleteTopic(topic);
      }
    } finally {
      connection.close();
    }
  });
}
