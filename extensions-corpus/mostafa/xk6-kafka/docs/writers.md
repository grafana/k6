# Manage writers (producers) with xk6-kafka

## What is the Writer?

The writer is a Kafka producer that allows you to send messages to Kafka topics in a load test using k6. It is configured based on the environment and application settings.
Once you've created the writer, you can use it to produce messages to Kafka topics.
Writers are unique for each topic, if you want to produce messages to multiple topics, you need to create a writer for each topic.

## Writers Configuration

### Create single writer

```javascript
const producer = new Writer({
  brokers, // The broker addresses as array, e.g., ["localhost:9092"]
  topic: topicName, // The topic name to produce messages to
  compression: CODEC_SNAPPY, // Optional: Compression codec to use for messages
  tls: tlsConfig, // Config object for TLS settings, if needed
});

producer.produce({
  messages: messagesToProduce,
});
```

**📌 Important**: Topics should be created in the `setup()` function before producing messages. See [Topic Management](#topic-management) below.

Tip: here you can find more options for your writer:

```golang
type WriterConfig struct {
	AutoCreateTopic bool          `json:"autoCreateTopic"`
	ConnectLogger   bool          `json:"connectLogger"`
	MaxAttempts     int           `json:"maxAttempts"`
	BatchSize       int           `json:"batchSize"`
	BatchBytes      int           `json:"batchBytes"`
	RequiredAcks    int           `json:"requiredAcks"`
	Topic           string        `json:"topic"`
	Balancer        string        `json:"balancer"`
	Compression     string        `json:"compression"`
	Brokers         []string      `json:"brokers"`
	BatchTimeout    time.Duration `json:"batchTimeout"`
	ReadTimeout     time.Duration `json:"readTimeout"`
	WriteTimeout    time.Duration `json:"writeTimeout"`
	SASL            SASLConfig    `json:"sasl"`
	TLS             TLSConfig     `json:"tls"`
}
```

All the parameters are named following the camelCase notation style.

### Manage multiple writers for various topics

A lot of times, you will need to produce messages to multiple topics. In this case, you can create a writer for each topic and manage them in an object or a map.

```javascript
function createKafkaWriters(brokers, topicNamesList, tlsConfig) {
  const writers = new Map();
  Object.values(topicNamesList).forEach((topicName) => {
    writers.set(
      topicName,
      new Writer({
        brokers,
        topic: topicName,
        autoCreateTopic: false,
        compression: CODEC_SNAPPY,
        tls: tlsConfig,
      }),
    );
  });
  return writers;
}
```

You can then use the `writers` map to produce messages to the topics:

```javascript
const producers = createKafkaWriters(brokers, topicNamesList, tlsConfig);
producers.get(topicName).produce({
  messages: messagesToProduce,
});
```

## Message Production

You can see above that the `produce` method is used to send messages to the Kafka topic.
The `messages` parameter is an array of messages you want to send.
Each message can be a simple string or an object, depending on your Kafka topic configuration.

You can here find an example of how to create a messages:

### Create a JSON message

```javascript
let messages = [
  {
    // The data type of the key is JSON
    key: schemaRegistry.serialize({
      data: {
        correlationId: "test-id-abc-" + index,
      },
      schemaType: SCHEMA_TYPE_JSON,
    }),
    // The data type of the value is JSON
    value: schemaRegistry.serialize({
      data: {
        name: "xk6-kafka",
        version: "0.9.0",
        author: "Mostafa Moradian",
        description:
          "k6 extension to load test Apache Kafka with support for Avro messages",
        index: index,
      },
      schemaType: SCHEMA_TYPE_JSON,
    }),
    headers: {
      mykey: "myvalue",
    },
    offset: index,
    partition: 0,
    time: new Date(), // Will be converted to timestamp automatically
  },
];
```

### Create an AVRO message

For this usage, you'll need to have the Avro schema registered in your Schema Registry.
Please refer to the [Schema Registry documentation](./schema-registry.md) for more details on how to register schemas.

```javascript
let messages = [
  {
    key: schemaRegistry.serialize({
      data: {
        correlationId: "test-id-abc-" + index,
      },
      schema: myAvroKeySchema, // The Javascript object containing Avro schema for the key
      schemaType: SCHEMA_TYPE_AVRO,
    }),
    value: schemaRegistry.serialize({
      data: {
        name: "xk6-kafka",
        version: "0.9.0",
        author: "Mostafa Moradian",
        description:
          "k6 extension to load test Apache Kafka with support for Avro messages",
        index: index,
      },
      schema: myAvroValueSchema, // The Javascript object containing Avro schema for the key
      schemaType: SCHEMA_TYPE_AVRO,
    }), // Will be converted to timestamp automatically
  },
];
```

---

## Topic Management

### Why Topic Creation Matters

When running tests with multiple VUs (Virtual Users), topics must exist before messages are produced. Creating topics at the module level with `if (__VU == 0)` causes race conditions where other VUs start producing before the topic is ready.

### ✅ Correct Pattern: Use setup()

The `setup()` function runs **once** before any VU iterations begin, and **all VUs wait** for it to complete. This is the recommended way to create topics:

```javascript
import { sleep } from "k6";
import { Writer, Connection } from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "my-topic";

// Create Writer at module level
const writer = new Writer({
  brokers: brokers,
  topic: topic,
});

export function setup() {
  // Create Connection inside setup() for proper VU context
  const connection = new Connection({
    address: brokers[0],
  });

  connection.createTopic({
    topic: topic,
    numPartitions: 10,
    replicationFactor: 1,
  });

  // Verify topic was created
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  // This ensures Writer can see all partitions
  sleep(2);
}

export default function () {
  // Now it's safe to produce messages
  writer.produce({
    messages: [
      {
        key: "my-key",
        value: "my-value",
      },
    ],
  });
}

export function teardown() {
  // Clean up: delete topic and close writer
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
}
```

### Alternative: autoCreateTopic

If you don't need to control partition count, you can use `autoCreateTopic`:

```javascript
const writer = new Writer({
  brokers: brokers,
  topic: topic,
  autoCreateTopic: true, // Writer creates topic with broker defaults
});
```

**⚠️ Limitations of autoCreateTopic:**

- Uses broker's default partition count (often 1)
- Cannot specify replication factor
- Cannot set topic-level configurations

### ❌ Incorrect Pattern: Module-level with \_\_VU check

**Don't do this** - it causes race conditions:

```javascript
// ❌ BAD: Other VUs start before topic is ready
if (__VU == 0) {
  connection.createTopic({ topic: topic });
}
```

---
