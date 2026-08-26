// Compatibility fixture: modernized port of mostafa/xk6-kafka scripts/test_topics.js.
// Community v1 topic admin (create / list). Broker from KAFKA_BROKER; the
// setup's throw-on-missing is kept and a check() assertion added so it gates.
import { check, sleep } from "k6";
import { Connection } from "k6/x/kafka";
import { thresholds, runTest, getBroker } from "../lib/common.js";

const address = getBroker();
const topic = "xk6_kafka_test_topic";

export const options = { thresholds };

export function setup() {
  const connection = new Connection({ address });
  if (connection.listTopics().includes(topic)) {
    connection.deleteTopic(topic);
    sleep(2);
  }
  connection.createTopic({ topic });
  const topics = connection.listTopics();
  connection.close();
  sleep(2);
  return { topics };
}

export default function (data) {
  runTest(() => {
    check(data, {
      "created topic is listed": (d) => d.topics.includes(topic),
    });
  });
}

export function teardown() {
  const connection = new Connection({ address });
  connection.deleteTopic(topic);
  connection.close();
}
