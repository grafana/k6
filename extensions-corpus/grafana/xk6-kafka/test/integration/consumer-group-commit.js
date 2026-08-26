// Integration test: a consumer group commits its offsets on close, so a later
// member of the same group resumes after the committed offset instead of
// reprocessing from the start.
//
// Requires KAFKA_BROKER (run `make broker-up` or `make integration`).
import { Writer, Reader, Connection } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, toStr, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_group_commit");
    const group = uniqueTopic("xk6_grp");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        // Produce 6 messages: m0..m5.
        const writer = new Writer({ brokers: [broker], topic });
        try {
          const messages = [];
          for (let i = 0; i < 6; i++) {
            messages.push({ value: `m${i}` });
          }
          writer.produce({ messages });
        } finally {
          writer.close();
        }

        // First group member: consume 3 (m0..m2), then close — closing should
        // commit the consumed offsets.
        let first = [];
        const reader1 = new Reader({ brokers: [broker], topic, groupID: group });
        try {
          for (let i = 0; i < 5 && first.length < 3; i++) {
            first = first.concat(reader1.consume({ limit: 3, expectTimeout: true }));
          }
        } finally {
          reader1.close();
        }
        if (!verify("first member consumed 3 messages", first.length >= 3)) {
          return;
        }

        // Second member of the same group: must resume after the committed
        // offset (m3..m5), not reprocess m0.
        let second = [];
        const reader2 = new Reader({ brokers: [broker], topic, groupID: group });
        try {
          for (let i = 0; i < 5 && second.length < 3; i++) {
            second = second.concat(reader2.consume({ limit: 3, expectTimeout: true }));
          }
        } finally {
          reader2.close();
        }

        const firstValues = first.map((m) => toStr(m.value));
        const secondValues = second.map((m) => toStr(m.value));
        verify("second member resumed (did not reprocess the first message)",
          secondValues.indexOf(firstValues[0]) === -1);
        verify("second member consumed the remaining messages", second.length >= 3);
      } finally {
        connection.deleteTopic(topic);
      }
    } finally {
      connection.close();
    }
  });
}
