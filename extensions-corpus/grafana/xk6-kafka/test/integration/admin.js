// Integration test: topic administration against a real broker.
//
// create → list (present) → delete. The delete only asserts the broker accepts
// the request; Kafka removes the topic asynchronously, so this does not poll for
// the topic to disappear. Requires KAFKA_BROKER (run `make broker-up` or
// `make integration`).
import { Connection } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_admin");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        const after = connection.listTopics();
        verify("created topic is listed", after.includes(topic));
      } finally {
        // deleteTopic is both the operation under test and the cleanup: it only
        // runs once createTopic succeeded, so the topic is not left behind.
        connection.deleteTopic(topic);
      }
      verify("delete accepted", true);
    } finally {
      connection.close();
    }
  });
}
