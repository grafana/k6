/*

This is a k6 test script that imports the xk6-kafka and
tests Kafka with a 200 JSON messages per iteration. It
also uses SASL authentication.

*/

import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  SCHEMA_TYPE_JSON,
  SASL_PLAIN,
  TLS_1_2,
} from "k6/x/kafka"; // import kafka extension

export const options = {
  // This is used for testing purposes. For real-world use, you should use your own options:
  // https://k6.io/docs/using-k6/k6-options/
  scenarios: {
    sasl_auth: {
      executor: "constant-vus",
      vus: 1,
      duration: "10s",
      gracefulStop: "1s",
    },
  },
};

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_json_topic";

// SASL config is optional
const saslConfig = {
  username: "client",
  password: "client-secret",
  // Possible values for the algorithm is:
  // NONE (default)
  // SASL_PLAIN
  // SASL_SCRAM_SHA256
  // SASL_SCRAM_SHA512
  // SASL_SSL (must enable TLS)
  // SASL_AWS_IAM (configurable via env or AWS IAM config files - no username/password needed)
  algorithm: SASL_PLAIN,
};

// TLS config is optional
const tlsConfig = {
  // Enable/disable TLS (default: false)
  enableTls: false,
  // Skip TLS verification if the certificate is invalid or self-signed (default: false)
  insecureSkipTlsVerify: false,
  // Possible values:
  // TLS_1_0
  // TLS_1_1
  // TLS_1_2 (default)
  // TLS_1_3
  minVersion: TLS_1_2,

  // Only needed if you have a custom or self-signed certificate and keys
  // clientCertPem: "/path/to/your/client.pem",
  // clientKeyPem: "/path/to/your/client-key.pem",
  // serverCaPem: "/path/to/your/ca.pem",
};

const offset = 0;
// partition and groupId are mutually exclusive
const partition = 0;
const numPartitions = 1;
const replicationFactor = 1;

const writer = new Writer({
  brokers: brokers,
  topic: topic,
  sasl: saslConfig,
  tls: tlsConfig,
});
const reader = new Reader({
  brokers: brokers,
  topic: topic,
  partition: partition,
  offset: offset,
  sasl: saslConfig,
  tls: tlsConfig,
});
const schemaRegistry = new SchemaRegistry();

export function setup() {
  const connection = new Connection({
    address: brokers[0],
    sasl: saslConfig,
    tls: tlsConfig,
  });

  connection.createTopic({
    topic: topic,
    numPartitions: numPartitions,
    replicationFactor: replicationFactor,
  });

  // Verify topic was created
  const topics = connection.listTopics(saslConfig, tlsConfig);
  console.log("Existing topics: ", topics);
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  sleep(2);
}

export default function () {
  for (let index = 0; index < 100; index++) {
    let messages = [
      {
        key: schemaRegistry.serialize({
          data: {
            correlationId: "test-id-abc-" + index,
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
        value: schemaRegistry.serialize({
          data: {
            name: "xk6-kafka",
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
      },
      {
        key: schemaRegistry.serialize({
          data: {
            correlationId: "test-id-def-" + index,
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
        value: schemaRegistry.serialize({
          data: {
            name: "xk6-kafka",
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
      },
    ];

    writer.produce({ messages: messages });
  }

  // Read 10 messages only
  let messages = reader.consume({ limit: 10 });
  check(messages, {
    "10 messages returned": (msgs) => msgs.length == 10,
    "key is correct": (msgs) =>
      schemaRegistry
        .deserialize({ data: msgs[0].key, schemaType: SCHEMA_TYPE_JSON })
        .correlationId.startsWith("test-id-"),
    "value is correct": (msgs) =>
      schemaRegistry.deserialize({
        data: msgs[0].value,
        schemaType: SCHEMA_TYPE_JSON,
      }).name == "xk6-kafka",
  });
}

export function teardown(data) {
  const connection = new Connection({
    address: brokers[0],
    sasl: saslConfig,
    tls: tlsConfig,
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
