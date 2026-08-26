# 📥 Manage Readers (Consumers) with xk6-kafka

## What is the Reader?

The reader is a Kafka consumer that allows you to **read/consume messages from Kafka topics** during a load or functional test using k6. Readers are useful for validating that a system under test is
producing expected events, or to simulate consuming behavior in performance scenarios.

Each reader listens to a specific topic and can be configured with several options like offset, partitions, group ID, and message limits.

---

## 🛠️ Reader Configuration

### Create a Single Reader

```javascript
const reader = new Reader({
  brokers, // Array of Kafka broker addresses
  topic: topicName, // The topic to consume from
  groupId: "my-consumer-group", // Consumer group ID
  partition: 0, // Optional: partition to read from, ineffective when using consumer group
  minBytes: 1, // Minimum bytes per fetch request
  maxBytes: 1048576, // Maximum bytes per fetch request
  offset: 0, // Optional: Start at specific offset (default: latest)
  tls: tlsConfig, // TLS configuration, if needed
});
```

Tip: here you can find more options for your reader:

```golang
type ReaderConfig struct {
	WatchPartitionChanges  bool          `json:"watchPartitionChanges"`
	ConnectLogger          bool          `json:"connectLogger"`
	Partition              int           `json:"partition"`
	QueueCapacity          int           `json:"queueCapacity"`
	MinBytes               int           `json:"minBytes"`
	MaxBytes               int           `json:"maxBytes"`
	MaxAttempts            int           `json:"maxAttempts"`
	GroupID                string        `json:"groupId"`
	Topic                  string        `json:"topic"`
	IsolationLevel         string        `json:"isolationLevel"`
	StartOffset            string        `json:"startOffset"`
	Offset                 int64         `json:"offset"`
	Brokers                []string      `json:"brokers"`
	GroupTopics            []string      `json:"groupTopics"`
	GroupBalancers         []string      `json:"groupBalancers"`
	MaxWait                Duration      `json:"maxWait"`
	ReadBatchTimeout       time.Duration `json:"readBatchTimeout"`
	ReadLagInterval        time.Duration `json:"readLagInterval"`
	HeartbeatInterval      time.Duration `json:"heartbeatInterval"`
	CommitInterval         time.Duration `json:"commitInterval"`
	PartitionWatchInterval time.Duration `json:"partitionWatchInterval"`
	SessionTimeout         time.Duration `json:"sessionTimeout"`
	RebalanceTimeout       time.Duration `json:"rebalanceTimeout"`
	JoinGroupBackoff       time.Duration `json:"joinGroupBackoff"`
	RetentionTime          time.Duration `json:"retentionTime"`
	ReadBackoffMin         time.Duration `json:"readBackoffMin"`
	ReadBackoffMax         time.Duration `json:"readBackoffMax"`
	OffsetOutOfRangeError  bool          `json:"offsetOutOfRangeError"` // deprecated, do not use
	SASL                   SASLConfig    `json:"sasl"`
	TLS                    TLSConfig     `json:"tls"`
}
```

All the parameters are named following the camelCase notation style.

---

### Read Messages

Once you’ve created the reader, you can consume messages using the `consume()` method.

```javascript
const messages = reader.consume({
  limit: 10, // Max number of messages to consume
  expectedTimeout: "2s", // Max time to wait
});
```

---

## 🧵 Managing Multiple Readers

If you need to consume from several topics simultaneously, you can manage multiple readers in a `Map`.

```javascript
function createKafkaReaders(brokers, topicNamesList, tlsConfig) {
  const readers = new Map();
  Object.values(topicNamesList).forEach((topicName) => {
    readers.set(
      topicName,
      new Reader({
        brokers,
        topic: topicName,
        groupId: "my-group",
        offset: 0,
        tls: tlsConfig,
      }),
    );
  });
  return readers;
}
```

You can then use them like this:

```javascript
const readers = createKafkaReaders(brokers, topicNamesList, tlsConfig);
const messages = readers.get("my-topic").consume({
  limit: 5,
  expectedTimeout: "2s",
});
```

Returned `messages` is an array of consumed messages, each with properties such as `key`, `value`, `timestamp`, `offset`, etc.

---

## Deserializing Messages

If you are using a schema registry, you'll have to deserialize the messages after consuming them. Here’s how you can do it:
You need first to create a `SchemaRegistry` instance. Please refer to the [Schema Registry documentation](./schema-registry.md) for more details on how to set it up.
In order to perform deserialization, you can use the `schemaRegistry.deserialize` method:

```javascript
const readers = createKafkaReaders(brokers, topicNamesList, tlsConfig);
const messages = readers.get("my-topic").consume({
  limit: 5,
  expectedTimeout: "2s",
});
```

---

## ✅ Notes

- The `offset` can be set to:
  - `0` to start from the beginning
  - `-1` to start from the latest message
- `groupId` ensures offset tracking and distribution in real Kafka clusters.
- Be careful when consuming a high volume — make sure the `expectedTimeout` and `limit` values are tuned properly for your tests.

---

## 🔧 Topic Management for Readers

Before consuming messages, ensure the topic exists. When running tests with multiple VUs, create topics in the `setup()` function:

```javascript
import { sleep } from "k6";
import { Reader, Connection } from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "my-topic";

// Create Reader at module level
const reader = new Reader({
  brokers: brokers,
  topic: topic,
});

export function setup() {
  // Create Connection inside setup()
  const connection = new Connection({
    address: brokers[0],
  });

  // Ensure topic exists (or create it)
  connection.createTopic({
    topic: topic,
    numPartitions: 3,
  });

  // Verify topic was created
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate
  sleep(2);
}

export default function () {
  // Now it's safe to consume messages
  const messages = reader.consume({ limit: 10 });
  console.log(`Consumed ${messages.length} messages`);
}

export function teardown() {
  // Clean up
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  reader.close();
}
```

**Why `setup()`?**

- Runs once before all VUs start
- All VUs wait for it to complete
- Prevents race conditions
- Ensures metadata propagation with `sleep(2)`

For more details on topic management, see the [Writers documentation](./writers.md#topic-management).
