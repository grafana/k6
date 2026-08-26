import { check, sleep } from "k6";
import { Writer, Reader, Connection } from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topics = ["topicA", "topicB"];

// Create writers and readers for each topic
const writers = new Map();
const readers = new Map();

topics.forEach((topic) => {
  writers.set(
    topic,
    new Writer({
      brokers,
      topic,
      autoCreateTopic: false,
    }),
  );

  readers.set(
    topic,
    new Reader({
      brokers,
      topic,
    }),
  );
});

export function setup() {
  const connection = new Connection({ address: brokers[0] });

  for (const topic of topics) {
    connection.createTopic({ topic });
  }

  // Verify all topics were created
  const createdTopics = connection.listTopics();
  for (const topic of topics) {
    if (!createdTopics.includes(topic)) {
      throw new Error(`Topic ${topic} was not created successfully`);
    }
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  sleep(2);
}

export default function () {
  for (const topic of topics) {
    const writer = writers.get(topic);

    // Produce 5 messages to each topic
    const messages = Array.from({ length: 5 }, (_, i) => ({
      key: `key-${topic}-${i}`,
      value: `value-${topic}-${i}`,
    }));

    writer.produce({ messages });

    // Read back 5 messages from each topic
    const reader = readers.get(topic);
    const consumed = reader.consume({ limit: 5 });

    check(consumed, {
      [`${topic}: 5 messages read`]: (msgs) => msgs.length === 5,
      [`${topic}: first key correct`]: (msgs) =>
        msgs[0].key.includes(`key-${topic}`),
    });
  }
}

export function teardown() {
  const connection = new Connection({ address: brokers[0] });

  for (const topic of topics) {
    connection.deleteTopic(topic);
  }

  connection.close();

  // Close all clients
  writers.forEach((w) => w.close());
  readers.forEach((r) => r.close());
}
