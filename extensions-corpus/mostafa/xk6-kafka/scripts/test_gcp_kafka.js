/*
This is a k6 test script that imports the xk6-kafka and
tests GCP Kafka with 1 string message per iteration. It
also uses GCP OAuth authentication.

The small number and size of messages per iteration is to 
reduce cost for smoke testing by allowing a smaller cluster 
to be used.

If you are benchmarking a high scale use case, you can 
adjust the test settings to accomodate your throughput 
requirements.
*/

import { check } from "k6";
import {
  Producer,
  Consumer,
  AdminClient,
  SchemaRegistry,
  SCHEMA_TYPE_STRING,
  SASL_GCP_OAUTH,
  TLS_1_2,
} from "k6/x/kafka";

if (!__ENV.GCP_KAFKA_BOOTSTRAP_SERVER) {
  throw new Error(
    `Environment variable GCP_KAFKA_BOOTSTRAP_SERVER is missing!`,
  );
}

const brokers = [`${__ENV.GCP_KAFKA_BOOTSTRAP_SERVER}`];
const topic = "k6-gcp-kafka-test";
const numPartitions = 1;
const groupId = "k6";

const saslConfig = {
  algorithm: SASL_GCP_OAUTH,
};

const tlsConfig = {
  enableTls: true,
  insecureSkipTlsVerify: false,
  minVersion: TLS_1_2,
};

const producer = new Producer({
  brokers: brokers,
  topic: topic,
  sasl: saslConfig,
  tls: tlsConfig,
});

const consumer = new Consumer({
  brokers: brokers,
  topic: topic,
  groupId: groupId,
  sasl: saslConfig,
  tls: tlsConfig,
  // Need to allow time for rebalance
  maxWait: "30s",
});

const adminClient = new AdminClient({
  brokers: brokers,
  sasl: saslConfig,
  tls: tlsConfig,
});

const schemaRegistry = new SchemaRegistry();

export function setup() {
  try {
    adminClient.createTopic({ topic: topic });
  } catch {
    // topic already exists
  }
}

export default function main() {
  produce();
  consume();
}

function produce() {
  for (let i = 0; i < 10; i++) {
    let messages = [
      {
        key: schemaRegistry.serialize({
          data: String(i),
          schemaType: SCHEMA_TYPE_STRING,
        }),
        value: schemaRegistry.serialize({
          data: `Hello, GCP Kafka!`,
          schemaType: SCHEMA_TYPE_STRING,
        }),
      },
    ];

    producer.produce({ messages: messages });
  }
}

function consume() {
  let messages = consumer.consume({ limit: 10 });

  check(messages, {
    "10 messages returned": (msgs) => msgs.length == 10,
    "key is correct": (msgs) =>
      schemaRegistry.deserialize({
        data: msgs[0].key,
        schemaType: SCHEMA_TYPE_STRING,
      }) != undefined,
    "value is correct": (msgs) =>
      schemaRegistry.deserialize({
        data: msgs[0].value,
        schemaType: SCHEMA_TYPE_STRING,
      }) == "Hello, GCP Kafka!",
  });
}

export function teardown(data) {
  adminClient.deleteTopic(topic);
  adminClient.close();
  producer.close();
  consumer.close();
}
